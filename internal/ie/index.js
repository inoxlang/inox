import "./editor/codemirror.js";

//SHOW HINT
import "./editor/addon/hint/show-hint.js";

//JS
import "./editor/mode/javascript/javascript.js";
import "./editor/addon/hint/javascript-hint.js";


//INOX
import "./editor/mode/inox/inox.js";

//LSP
import { CodeMirrorAdapter } from "./editor/lsp/lsp-adapter.js";
import { LspConnection } from "./editor/lsp/connection.js";

const inoxWorker = new Worker("./worker.js", {
  type: "module",
});

  /** @type {Record<string, ((response: unknown) => any)} */
let responseCallbacks = {};

/** 
 * @param {string} method
 * @param {unknown} args
 */
const sendRequestToInoxWorker = async (method, args) => {
  let id = Math.random() 
  inoxWorker.postMessage({ method, args, id});
  return new Promise((resolve, reject) => {
    setTimeout(reject, 2000)
    responseCallbacks[id] = resolve
  })
}

{

  inoxWorker.addEventListener("message", (ev) => {
    if (ev.data == "initialized") {
      setupLSP();
      return
    }

    let {method, id, response} = ev.data
    if(typeof id !== undefined){ //response
      let callback = responseCallbacks[id]
      if(callback){
        delete responseCallbacks[id]
        callback(response)
      } else {
        console.error('no response callback for reques with id', id)
      }
    } else { //notification 

    }
  
  });
}


let editor = CodeMirror(document.querySelector("#editor-wrapper"), {
  value: "fn myScript(){return 100;}\n",
  mode: "inox",
  extraKeys: {
    "Ctrl-Space": "autocomplete",
  },
});

async function setupLSP() {
  console.info("setup LSP client");

  /** @type {ILspOptions} */
  let lspOptions = {
    documentText: () => editor.getDoc().getValue(),
    documentUri: "file:///script.ix",
    languageId: "inox",
    rootUri: "file:///",
    serverUri: "",
  };

  /** @param {string} data */
  const writeInput = (data) => sendRequestToInoxWorker(
    "write_lsp_input",
    { input: data }
  );

  /** @returns {string} */
  const readOutput = async () => {
    let s = await sendRequestToInoxWorker("read_lsp_output");
    if(typeof s != 'string'){
      console.error('a string was expected')
      return '';
    }
    return s
  };

  let conn = new LspConnection(lspOptions, writeInput, readOutput);
  conn.connect();

  /** @type {ITextEditorOptions} */
  let adapterOptions = {};
  new CodeMirrorAdapter(conn, adapterOptions, editor);
}
