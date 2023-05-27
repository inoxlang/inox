import { Go } from './wasm_exec.js';
import './editor/codemirror.js'
//JS
import './editor/mode/javascript/javascript.js'
import './editor/addon/hint/javascript-hint.js'
//INOX
import './editor/mode/inox/inox.js'

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

WebAssembly.instantiateStreaming(
  fetch("browser-lsp-server.wasm"),
  go.importObject,
).then(
  async result => {
    mod = result.module;
    inst = result.instance;

    go.run(inst);

    setTimeout(() => {
      let exports = /** 
      * @type { {
      * 	setup: (arg: {IWD: string}) => any,
      *  write_lsp_input: (s: string) => void,
      *  read_lsp_output: () => string
      * }} */ (go.exports);
  
      exports.setup({
        IWD: '/home/user'
      })
  
      exports.write_lsp_input('HEEEZBEJBZEJBJEBJZEBJZBEJZEBJZEBJZBEJZEBJZEBJZEBJZEZBEJZEZEEZLLO')
  
      //@ts-ignore
      globalThis['exports'] = exports;


      setupEditor()
    }, 10)
  },
);


function setupEditor(){
  var myCodeMirror = CodeMirror(document.body, {
    value: "fn myScript(){return 100;}\n",
    mode:  "inox",
    extraKeys: {
      "Ctrl-Space": "autocomplete"
    }
  });
}