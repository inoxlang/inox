import { Go } from './wasm_exec.js';
import './editor/codemirror.js'

//JS
import './editor/mode/javascript/javascript.js'
import './editor/addon/hint/javascript-hint.js'

//INOX
import './editor/mode/inox/inox.js'

//LSP
import {CodeMirrorAdapter} from './editor/lsp/lsp-adapter.js' 
import {LspConnection} from './editor/lsp/connection.js' 


//polyfill for WebAssembly.instantiateStreaming
if (!WebAssembly.instantiateStreaming) {
  WebAssembly.instantiateStreaming = async (resp, importObject) => {
    const source = await (await resp).arrayBuffer();
    return await WebAssembly.instantiate(source, importObject);
  };
}

const go = new Go();

/** @type {WebAssembly.Module} */
let mod;

/** @type {WebAssembly.Instance} */
let inst;

/** 
* @typedef { {
* 	setup: (arg: {IWD: string, print_debug: Function}) => any,
*   write_lsp_input: (s: string) => void,
*   read_lsp_output: () => string
* }} InoxExports
*/

WebAssembly.instantiateStreaming(
  fetch("browser-lsp-server.wasm"),
  go.importObject,
).then(
  async result => {
    mod = result.module;
    inst = result.instance;

    go.run(inst);

    setTimeout(() => {
      let exports = /** @type {InoxExports} */ (go.exports);
  
      exports.setup({
        IWD: '/home/user',
        print_debug: console.debug
      })
  
      setupEditor(exports)
    }, 10)
  },
);


/** @param {InoxExports} exports */
function setupEditor(exports){
  let editor = CodeMirror(document.body, {
    value: "fn myScript(){return 100;}\n",
    mode:  "inox",
    extraKeys: {
      "Ctrl-Space": "autocomplete"
    }
  });

  /** @type {ILspOptions} */
  let lspOptions = {
    documentText: () => editor.getDoc().getValue(),
    documentUri: 'file:///script.ix',
    languageId: 'inox',
    rootUri: 'file:///',
    serverUri: ''
  }

  let conn = new LspConnection(lspOptions, exports.write_lsp_input, exports.read_lsp_output)
  conn.connect()


  /** @type {ITextEditorOptions} */
  let adapterOptions = {}
  new CodeMirrorAdapter(conn, adapterOptions, editor)
}