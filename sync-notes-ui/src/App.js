import './App.css';
import React, {useEffect, useState} from "react";
import wretch from "wretch";
import MarkdownEditor from "rich-markdown-editor";

const BASE_URL = process.env.REACT_APP_BASE_URL;

export function App() {
    const [note, setNote] = useState();

    useEffect(async () => {
        const { id } = await wretch(`${BASE_URL}/v1/create-note-request`).post().json();
        setNote({ id })
    }, []);

    if (!note) {
        return <div>Loading...</div>
    }

    const onSaveClick = async () => {
        // Setting this to null triggers the loading state
        setNote(null);
        setNote(await save(ref, note));
    }

    const ref = React.createRef();
    return (
        <div>
            <div className="header">
                Sync Notes
                <button role="button" onClick={onSaveClick}>Save</button>
            </div>
            <div className="editor-container">
                <MarkdownEditor value={note.data} ref={ref}/>
            </div>
        </div>
    );

}

async function save(editorRef, note) {
    const value = editorRef.current.value();
    if (note.data === undefined) {
        return await wretch(`${BASE_URL}/v1/note`)
            .post({
                id: note.id,
                data: value
            })
            .json();
    } else {
        return await wretch(`${BASE_URL}/v1/note/${note.id}`)
            .put({
                id: note.id,
                data: value
            })
            .json();
    }
}
