import { Go } from './wasm_exec.js';

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
        IWD: '/',
        print_debug: console.debug
      })

      self.postMessage('initialized')
      
      self.addEventListener('message', ev => {
        let {method, args, id} = ev.data


        switch(method){
        case "write_lsp_input": {
          if(id === undefined){
            console.error('missing .id in call to write_lsp_input')
          }

          let input = args.input
          if(input === undefined){
            console.error('missing .input argument in call to write_lsp_input')
          } else {
            exports.write_lsp_input(String(input))
            self.postMessage({ method, id, response: null })
          }

          break
        }
        case "read_lsp_output": {
          if(id === undefined){
            console.error('missing .id in call to read_lsp_output')
          }
          let output = exports.read_lsp_output()
          self.postMessage({ method, id, response: output })

          break
        }
        default:
          console.error('unknown method ' + method)
        }


      })
    }, 10)
  },
);

