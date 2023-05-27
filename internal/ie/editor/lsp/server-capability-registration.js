//slightly modified version of:
//https://github.com/wylieconlon/lsp-editor-adapter/blob/master/src/server-capability-registration.ts by Wylie Conlon (ISC license)


/** @typedef {import('vscode-languageserver-protocol').Registration}  Registration */
/** @typedef {import('vscode-languageserver-protocol').ServerCapabilities} ServerCapabilities */
/** @typedef {import('vscode-languageserver-protocol').Unregistration} Unregistration */

const ServerCapabilitiesProviders = {
  'textDocument/hover': 'hoverProvider',
  'textDocument/completion': 'completionProvider',
  'textDocument/signatureHelp': 'signatureHelpProvider',
  'textDocument/definition': 'definitionProvider',
  'textDocument/typeDefinition': 'typeDefinitionProvider',
  'textDocument/implementation': 'implementationProvider',
  'textDocument/references': 'referencesProvider',
  'textDocument/documentHighlight' : 'documentHighlightProvider',
  'textDocument/documentSymbol' : 'documentSymbolProvider',
  'textDocument/workspaceSymbol' : 'workspaceSymbolProvider',
  'textDocument/codeAction' : 'codeActionProvider',
  'textDocument/codeLens' : 'codeLensProvider',
  'textDocument/documentFormatting' : 'documentFormattingProvider',
  'textDocument/documentRangeFormatting' : 'documentRangeFormattingProvider',
  'textDocument/documentOnTypeFormatting' : 'documentOnTypeFormattingProvider',
  'textDocument/rename' : 'renameProvider',
  'textDocument/documentLink' : 'documentLinkProvider',
  'textDocument/color' : 'colorProvider',
  'textDocument/foldingRange' : 'foldingRangeProvider',
  'textDocument/declaration' : 'declarationProvider',
  'textDocument/executeCommand' : 'executeCommandProvider',
};

/** 
 * @param {ServerCapabilities} serverCapabilities
 * @param {Registration} serverCapabilities
 * @returns {ServerCapabilities}
 */
function registerServerCapability(serverCapabilities, registration) {
  const serverCapabilitiesCopy = JSON.parse(JSON.stringify(serverCapabilities));
  const { method, registerOptions } = registration;
  const providerName = ServerCapabilitiesProviders[method];

  if (providerName) {
    if (!registerOptions) {
      serverCapabilitiesCopy[providerName] = true;
    } else {
      serverCapabilitiesCopy[providerName] = Object.assign({}, JSON.parse(JSON.stringify(registerOptions)));
    }
  } else {
    throw new Error('Could not register server capability.');
  }

  return serverCapabilitiesCopy;
}

/**
 * @param {ServerCapabilities} serverCapabilities 
 * @param {Unregistration} unregistration 
 * @returns {ServerCapabilities}
 */
function unregisterServerCapability(serverCapabilities, unregistration) {
  const serverCapabilitiesCopy = JSON.parse(JSON.stringify(serverCapabilities))
  const { method } = unregistration;
  const providerName = ServerCapabilitiesProviders[method];

  delete serverCapabilitiesCopy[providerName];

  return serverCapabilitiesCopy;
}

export {
  registerServerCapability,
  unregisterServerCapability,
};
