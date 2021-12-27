package main

import (
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/google/uuid"
	"log"
	"net/http"
	"os"
	"path"
	"time"
)

var storage = DiskStorage{"data"}
const storageAliveTime = 672 * time.Hour  // 28 days

var requestsCache = make(map[uuid.UUID]time.Time)
const requestCacheTimeout = 1 * time.Hour

func init() {
	err := storage.Init()
	if err != nil {
		panic(err)
	}

	// GC before we get going to clean things up
	storage.Gc(time.Now(), storageAliveTime)

	// Now set a timer to run the GC at a regular interval
	storageTicker := time.NewTicker(5 * time.Minute)
	go func() {
		for {
			<-storageTicker.C
			storage.Gc(time.Now(), storageAliveTime)
		}
	}()


	cacheTicker := time.NewTicker(5 * time.Minute)
	go func() {
		for {
			<-cacheTicker.C
			now := time.Now()
			for request, createTime := range requestsCache {
				if createTime.Add(requestCacheTimeout).Before(now) {
					log.Printf("deleting cached request %v initialised at %v\n", request, createTime)
					delete(requestsCache, request)
				}
			}
		}
	}()
}

func main() {


	r := chi.NewRouter()
	r.Use(middleware.Logger)

	// Basic CORS
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	}))

	// All responses are in JSON format
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			next.ServeHTTP(w, r)
		})
	})

	r.Post("/v1/create-note-request", func(w http.ResponseWriter, r *http.Request) {
		c := CreateNoteRequest{Id: uuid.New()}
		requestsCache[c.Id] = time.Now()

		w.WriteHeader(http.StatusCreated)
		e := json.NewEncoder(w)
		err := e.Encode(c)

		if err != nil {
			log.Printf("Error while writing response: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	})

	r.Post("/v1/note", func(w http.ResponseWriter, r *http.Request) {
		d := json.NewDecoder(r.Body)
		var note Note
		err := d.Decode(&note)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		_, ok := requestsCache[note.Id]
		if !ok {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		err = storage.SaveNote(note)
		if err != nil {
			log.Printf("Error while saving note: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		delete(requestsCache, note.Id)

		w.WriteHeader(http.StatusCreated)
		e := json.NewEncoder(w)
		err = e.Encode(note)
		if err != nil {
			log.Printf("Error while writing response: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	})
	r.Get("/v1/note/{id}", func(w http.ResponseWriter, r *http.Request) {
		urlId := chi.URLParam(r, "id")
		id, err := uuid.Parse(urlId)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		note, ok := storage.GetNote(id)
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.WriteHeader(http.StatusOK)
		e := json.NewEncoder(w)
		err = e.Encode(note)
		if err != nil {
			log.Printf("Error while writing response: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	})
	r.Put("/v1/note/{id}", func(w http.ResponseWriter, r *http.Request) {
		urlId := chi.URLParam(r, "id")
		id, err := uuid.Parse(urlId)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		d := json.NewDecoder(r.Body)
		var note Note
		err = d.Decode(&note)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if id != note.Id || !storage.DoesNoteExist(id) {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		err = storage.SaveNote(note)
		if err != nil {
			log.Printf("Error while saving note: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		e := json.NewEncoder(w)
		err = e.Encode(note)
		if err != nil {
			log.Printf("Error while writing response: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	})

	if err := http.ListenAndServe(":3000", r); err != nil {
		panic(err)
	}
}

type CreateNoteRequest struct {
	Id uuid.UUID `json:"id"`
}

type Note struct {
	Id   uuid.UUID `json:"id"`
	Data string    `json:"data"`
}

type DiskStorage struct {
	Directory string
}

func (d DiskStorage) Init() error {
	return os.MkdirAll(d.Directory, 0775)
}

func (d DiskStorage) DoesNoteExist(id uuid.UUID) bool {
	filePath := path.Join(d.Directory, id.String())
	_, err := os.Stat(filePath)
	return err == nil
}

func (d DiskStorage) SaveNote(n Note) error {
	filePath := path.Join(d.Directory, n.Id.String())
	return os.WriteFile(filePath, []byte(n.Data), 0664)
}

func (d DiskStorage) GetNote(id uuid.UUID) (Note, bool) {
	filePath := path.Join(d.Directory, id.String())
	data, err := os.ReadFile(filePath)
	if err != nil {
		return Note{}, false
	}

	return Note{Id: id, Data: string(data)}, true
}

func (d DiskStorage) Gc(now time.Time, expiry time.Duration) {
	entries, err := os.ReadDir(d.Directory)
	if err != nil {
		log.Panicf("Error reading directory: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			log.Printf("Error reading file %s: %v", entry.Name(), err)
			continue
		}
		atime := accessTime(info)

		if atime.Add(expiry).After(now) {
			continue
		}

		log.Printf("Expiring file: %s", entry.Name())
		err = os.Remove(path.Join(d.Directory, entry.Name()))
		if err != nil {
			log.Printf("Error removing file %s: %v", entry.Name(), err)
			continue
		}
	}
}
