import { Go } from "./wasm_exec.js";

//polyfill
if (!WebAssembly.instantiateStreaming) {
  // polyfill
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
    await go.run(inst);
    inst = await WebAssembly.instantiate(mod, go.importObject); // reset instance

    exports.setup({
      IWD: '/home/user'
    })
  },
);

