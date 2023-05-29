// modified version of:
//https://github.com/wylieconlon/lsp-editor-adapter/blob/master/src/ws-connection.ts by Wylie Conlon (ISC license)

/// <reference types="./lsp-types.d.ts"/>

import * as events from "./events.js";
import {
  registerServerCapability,
  unregisterServerCapability,
} from "./server-capability-registration.js";

const NOT_CONNECTED = "not connected";
const CONTENT_LENGTH_HEADER = "content-length: ";
const LOOP_WAIT_MILLIS = 5;

/** @typedef {import('vscode-languageserver-protocol').ClientCapabilities} ClientCapabilities */
/** @typedef {import('vscode-languageserver-protocol').ServerCapabilities} ServerCapabilities */
/** @typedef {import('vscode-languageserver-protocol').InitializeParams} InitializeParams */
/** @typedef {import('vscode-languageserver-protocol').InitializeResult} InitializeResult */
/** @typedef {import('vscode-languageserver-protocol').Registration} Registration */
/** @typedef {import('vscode-languageserver-protocol').Unregistration} Unregistration */

/** @typedef {import('vscode-languageserver-protocol').TextDocumentPositionParams} TextDocumentPositionParams */
/** @typedef {import('vscode-languageserver-protocol').DidChangeTextDocumentParams} DidChangeTextDocumentParams */
/** @typedef {import('vscode-languageserver-protocol').DidOpenTextDocumentParams} DidOpenTextDocumentParams */
/** @typedef {import('vscode-languageserver-protocol').PublishDiagnosticsParams} PublishDiagnosticsParams */
/** @typedef {import('vscode-languageserver-protocol').SignatureHelpParams} SignatureHelpParams */
/** @typedef {import('vscode-languageserver-protocol').ShowMessageParams} ShowMessageParams */
/** @typedef {import('vscode-languageserver-protocol').RegistrationParams} RegistrationParams */
/** @typedef {import('vscode-languageserver-protocol').UnregistrationParams} UnregistrationParams */
/** @typedef {import('vscode-languageserver-protocol').ShowMessageRequestParams} ShowMessageRequestParams */

/** @typedef {import('vscode-languageserver-protocol').Hover} Hover */
/** @typedef {import('vscode-languageserver-protocol').CompletionList} CompletionList */
/** @typedef {import('vscode-languageserver-protocol').CompletionItem} CompletionItem */
/** @typedef {import('vscode-languageserver-protocol').SignatureHelp} SignatureHelp */
/** @typedef {import('vscode-languageserver-protocol').DocumentHighlight} DocumentHighlight */
/** @typedef {import('vscode-languageserver-protocol').Location} Location */
/** @typedef {import('vscode-languageserver-protocol').LocationLink} LocationLink */

/**
 * @interface ExtendedClientCapabilities
 * @extends ClientCapabilities
 * @property {boolean} [xfilesProvider]
 * @property {boolean} [xcontentProvider]
 */

/**
 * @typedef {((result: unknown, err: unknown) => any) & {timeoutHandle?: number}} RequestCallback
 * @typedef {((params: unknown) => any)} RequestHandler
 * @typedef {((params: unknown) => any)} NotificationHandler
 */

/** @implements {ILspConnection} */
export class LspConnection extends events.EventEmitter {
  isConnected = false;

  isInitialized = false;

  _close = false;

  /** @type {ILspOptions} */
  documentInfo;

  /** @type {ServerCapabilities} */
  serverCapabilities;

  documentVersion = 0;

  //JSON RPC

  /** @type {(arg: string) => Promise<void>} */
  writeToServer;

  /** @type {() => Promise<string>} readFromServer */
  readFromServer;

  /** @type {Record<string, RequestCallback>} */
  pendingRequestCallbacks = {};

  /** @type {Record<string, RequestHandler[]>} */
  incomingRequestHandlers = {};

  /** @type {Record<string, NotificationHandler[]>} */
  incomingNotificationHandlers = {};

  /**
   * @param {ILspOptions} options
   * @param {(arg: string) => Promise<void>} writeToServer
   * @param {() => Promise<string>} readFromServer
   */
  constructor(options, writeToServer, readFromServer) {
    super();
    this.documentInfo = options;
    this.writeToServer = writeToServer;
    this.readFromServer = readFromServer;
  }

  /**
   * @returns {this}
   */
  connect() {
    setTimeout(() => {
      this.isConnected = true;
      this.startLoopAsync();
    }, 0);

    setTimeout(() => {
      this.sendInitialize();
    }, 10);

    this.onNotification(
      "textDocument/publishDiagnostics",
      /** @param {PublishDiagnosticsParams} params */
      (params) => {
        this.emit("diagnostic", params);
      },
    );

    this.onNotification(
      "window/showMessage",
      /** @param {ShowMessageParams} params */
      (params) => {
        this.emit("logging", params);
      },
    );

    this.onRequest(
      "client/registerCapability",
      /** @param {RegistrationParams} params */
      (params) => {
        params.registrations.forEach(
          /** @param {Registration} capabilityRegistration */
          (capabilityRegistration) => {
            this.serverCapabilities = registerServerCapability(
              this.serverCapabilities,
              capabilityRegistration,
            );
          },
        );

        this.emit("logging", params);
      },
    );

    this.onRequest(
      "client/unregisterCapability",
      /** @param {UnregistrationParams} params */
      (params) => {
        params.unregisterations.forEach(
          /** @param {Unregistration} capabilityUnregistration */
          (capabilityUnregistration) => {
            this.serverCapabilities = unregisterServerCapability(
              this.serverCapabilities,
              capabilityUnregistration,
            );
          },
        );

        this.emit("logging", params);
      },
    );

    this.onRequest(
      "window/showMessageRequest",
      /** @param {ShowMessageRequestParams} params */
      (params) => {
        this.emit("logging", params);
      },
    );

    return this;
  }

  async startLoopAsync() {
    //infinite loop reading the output
    let serverOutput = "";
    while (!this._close) {
      //read as much as possible
      {
        let chunk = "";

        while ((chunk =  await this.readFromServer()) != "") {
          await sleepMillis(LOOP_WAIT_MILLIS);
          serverOutput += chunk;
        }
      }

      if (serverOutput.trim() == "") {
        serverOutput = "";
        await sleepMillis(LOOP_WAIT_MILLIS);
        continue;
      }

      //parse & handle a single request
      let originalServerOutput = serverOutput;

      if (!serverOutput.toLowerCase().startsWith(CONTENT_LENGTH_HEADER)) {
        console.error(
          'JSON RPC request not starting with "content-length:" ->',
          serverOutput,
        );
        await sleepMillis(LOOP_WAIT_MILLIS);
        continue;
      }

      serverOutput = serverOutput.slice(CONTENT_LENGTH_HEADER.length).trim();
      if (serverOutput.match(/^[ \t]*\d+\r\n\r\n/)) {
        const crIndex = serverOutput.indexOf("\r");

        let contentLength = Number.parseInt(
          serverOutput.slice(0, crIndex).trim(),
        );

        if (isNaN(contentLength)) {
          console.error(
            "JSON RPC request has invalid content-length header ->",
            originalServerOutput,
          );
          continue;
        }

        //remove header & linefeeds
        serverOutput = serverOutput.slice(crIndex + 4);

        let encodedContent = encodeString(serverOutput).slice(0, contentLength);
        let decodedContent = decodeString(encodedContent);

        //remove content from server output
        serverOutput = serverOutput.slice(decodedContent.length);

        try {
          let json = JSON.parse(decodedContent);
          this.handleRPC(json);
        } catch (err) {
          console.error(
            "error while parsing & handling JSON RPC request",
            err,
            "\nrequest:",
            decodedContent,
          );
        }
      } else {
        console.error(
          "JSON RPC request has invalid content-length header ->",
          originalServerOutput,
        );
        continue;
      }
    }
  }

  /** @param {unknown} json */
  handleRPC(json) {
    if (typeof json != "object" || json === null) {
      console.error("invalid JSON, should be an object:", json);
      return;
    }

    if (("id" in json) && json.id !== undefined) { //requests have an id
      //request reponse
      if (
        ("result" in json && json.result != undefined) ||
        (("error" in json) && json.error != undefined)
      ) {
        let id = String(json.id);
        let callback = this.pendingRequestCallbacks[id];
        if (callback === undefined) {
          console.error("get response for request of unknown id", id, json);
          return;
        }
        if (callback.timeoutHandle) {
          clearTimeout(callback.timeoutHandle);
        }
        delete this.pendingRequestCallbacks[id];

        //@ts-ignore
        let error = json.error;
        //@ts-ignore
        let result = json.result;

        callback(result, error);
      } else { //request sent by the server

        if(! ('method' in json) || (typeof json.method !== 'string')) {
          console.error("missing/invalid .method in request sent by server:", json);
          return
        }

        if(! ('params' in json)) {
          console.error("missing .params in request sent by server:", json);
          return
        }

        console.debug("receive request from the server", json.method);

        let handlers = this.incomingRequestHandlers[json.method]
        handlers.forEach(handler => handler(json.params))
      }
    } else { //notification sent by the server

      if(! ('method' in json) || (typeof json.method !== 'string')) {
        console.error("missing/invalid .method in notification sent by server:", json);
        return
      }

      if(! ('params' in json)) {
        console.error("missing .params in request notification by server:", json);
        return
      }

      console.debug("receive notifcation from the server", json.method);
      let handlers = this.incomingNotificationHandlers[json.method]
      handlers.forEach(handler => handler(json.params))
    }
  }

  /**
   * @param {string} method
   * @param {any} params
   * @returns {Promise<unknown>}
   */
  sendRequest(method, params) {
    console.debug("send request", method);

    let requestId = Math.random();

    let req = JSON.stringify({
      method: method,
      params: params,
      id: requestId,
    });

    this.sendJSON(req);

    let promise = new Promise((resolve, reject) => {
      /** @type {RequestCallback} */
      let callback = (result, error) => {
        if (error) {
          console.error(error);
          reject(error);
        } else {
          resolve(result);
        }
      };
      //callback.timeoutHandle =
      this.pendingRequestCallbacks[requestId] = callback;
    });

    return promise;
  }

  /**
   * @param {string} method
   * @param {any} params
   */
  sendNotification(method, params) {
    console.debug("send notification", method);

    let notification = JSON.stringify({
      method: method,
      params: params,
    });

    this.sendJSON(notification);
  }

  /** @param {string} json */
  sendJSON(json) {
    let req = "Content-Length: " +
      String(new TextEncoder().encode(json).byteLength) + "\r\n\r\n" +
      json;

    this.writeToServer(req);
  }

  /**
   * @param {string} method
   * @param {NotificationHandler} handler
   */
  onNotification(method, handler) {
    this.incomingNotificationHandlers[method] =
      this.incomingNotificationHandlers[method] || [];

    this.incomingNotificationHandlers[method].push(handler);
  }

  /**
   * @param {string} method
   * @param {RequestHandler} handler
   */
  onRequest(method, handler) {
    this.incomingRequestHandlers[method] =
      this.incomingRequestHandlers[method] || [];

    this.incomingRequestHandlers[method].push(handler);
  }

  getDocumentUri() {
    return this.documentInfo.documentUri;
  }

  sendInitialize() {
    if (!this.isConnected) {
      return;
    }

    /** @type {InitializeParams} */
    const message = {
      capabilities: {
        textDocument: {
          hover: {
            dynamicRegistration: true,
            contentFormat: ["plaintext", "markdown"],
          },
          synchronization: {
            dynamicRegistration: true,
            willSave: false,
            didSave: false,
            willSaveWaitUntil: false,
          },
          completion: {
            dynamicRegistration: true,
            completionItem: {
              snippetSupport: false,
              commitCharactersSupport: true,
              documentationFormat: ["plaintext", "markdown"],
              deprecatedSupport: false,
              preselectSupport: false,
            },
            contextSupport: false,
          },
          signatureHelp: {
            dynamicRegistration: true,
            signatureInformation: {
              documentationFormat: ["plaintext", "markdown"],
            },
          },
          declaration: {
            dynamicRegistration: true,
            linkSupport: true,
          },
          definition: {
            dynamicRegistration: true,
            linkSupport: true,
          },
          typeDefinition: {
            dynamicRegistration: true,
            linkSupport: true,
          },
          implementation: {
            dynamicRegistration: true,
            linkSupport: true,
          },
        },
        workspace: {
          didChangeConfiguration: {
            dynamicRegistration: true,
          },
        },
        // xfilesProvider: true,
        // xcontentProvider: true,
      },
      initializationOptions: null,
      processId: null,
      rootUri: this.documentInfo.rootUri,
      workspaceFolders: null,
    };

    this.sendRequest("initialize", message).then(
      /** @param {InitializeResult} params */
      (params) => {
        this.isInitialized = true;
        this.serverCapabilities = params.capabilities;
        const textDocumentMessage = {
          textDocument: {
            uri: this.documentInfo.documentUri,
            languageId: this.documentInfo.languageId,
            text: this.documentInfo.documentText(),
            version: this.documentVersion,
          },
        };
        this.sendNotification("initialized");
        this.sendNotification("workspace/didChangeConfiguration", {
          settings: {},
        });
        this.sendNotification("textDocument/didOpen", textDocumentMessage);
        this.sendChange();
      },
      (e) => {
      },
    );
  }

  sendChange() {
    if (!this.isConnected) {
      return;
    }
    /** @type {DidChangeTextDocumentParams} */
    const textDocumentChange = {
      textDocument: {
        uri: this.documentInfo.documentUri,
        version: this.documentVersion,
      },
      contentChanges: [{
        text: this.documentInfo.documentText(),
      }],
    };
    this.sendNotification("textDocument/didChange", textDocumentChange);
    this.documentVersion++;
  }

  /** @param {IPosition} location */
  getHoverTooltip(location) {
    if (!this.isInitialized) {
      return;
    }
    this.sendRequest("textDocument/hover", {
      textDocument: {
        uri: this.documentInfo.documentUri,
      },
      position: {
        line: location.line,
        character: location.ch,
      },
    }).then(
      /** @param {Hover} params */
      (params) => {
        this.emit("hover", params);
      },
    );
  }

  /**
   * @param {IPosition} location
   * @param {ITokenInfo} token
   * @param {string|undefined} triggerCharacter
   * @param {CompletionTriggerKind|undefined} triggerKind
   * @returns
   */
  getCompletion(location, token, triggerCharacter, triggerKind) {
    if (!this.isConnected) {
      return;
    }
    if (
      !(this.serverCapabilities && this.serverCapabilities.completionProvider)
    ) {
      return;
    }

    this.sendRequest("textDocument/completion", {
      textDocument: {
        uri: this.documentInfo.documentUri,
      },
      position: {
        line: location.line,
        character: location.ch,
      },
      context: {
        triggerKind: triggerKind || CompletionTriggerKind.Invoked,
        triggerCharacter,
      },
    }).then(
      /**
       * @param { CompletionList | CompletionItem[] | null} params
       */
      (params) => {
        if (!params) {
          this.emit("completion", params);
          return;
        }
        this.emit("completion", "items" in params ? params.items : params);
      },
    );
  }

  /** @param {CompletionItem} completionItem */
  getDetailedCompletion(completionItem) {
    if (!this.isConnected) {
      return;
    }
    this.sendRequest("completionItem/resolve", completionItem)
      .then(
        /** @param {CompletionItem} result */
        (result) => {
          this.emit("completionResolved", result);
        },
      );
  }

  /**
   * @param {IPosition} location
   */
  getSignatureHelp(location) {
    if (!this.isConnected) {
      return;
    }
    if (
      !(this.serverCapabilities &&
        this.serverCapabilities.signatureHelpProvider)
    ) {
      return;
    }

    const code = this.documentInfo.documentText();
    const lines = code.split("\n");
    const typedCharacter = lines[location.line][location.ch];

    if (
      this.serverCapabilities.signatureHelpProvider &&
      !this.serverCapabilities.signatureHelpProvider.triggerCharacters.indexOf(
        typedCharacter,
      )
    ) {
      // Not a signature character
      return;
    }

    this.sendRequest("textDocument/signatureHelp", {
      textDocument: {
        uri: this.documentInfo.documentUri,
      },
      position: {
        line: location.line,
        character: location.ch,
      },
    }).then(
      /** @param {SignatureHelp} params */
      (params) => {
        this.emit("signature", params);
      },
    );
  }

  /**
   * Request the locations of all matching document symbols
   * @param {IPosition} location
   */
  getDocumentHighlights(location) {
    if (!this.isConnected) {
      return;
    }
    if (
      !(this.serverCapabilities &&
        this.serverCapabilities.documentHighlightProvider)
    ) {
      return;
    }

    this.sendRequest("textDocument/documentHighlight", {
      textDocument: {
        uri: this.documentInfo.documentUri,
      },
      position: {
        line: location.line,
        character: location.ch,
      },
    }).then(
      /** @param {DocumentHighlight[]} params */
      (params) => {
        this.emit("highlight", params);
      },
    );
  }

  /**
   * Request a link to the definition of the current symbol. The results will not be displayed
   * unless they are within the same file URI
   * @param {IPosition} location
   */
  getDefinition(location) {
    if (!this.isConnected || !this.isDefinitionSupported()) {
      return;
    }

    this.sendRequest("textDocument/definition", {
      textDocument: {
        uri: this.documentInfo.documentUri,
      },
      position: {
        line: location.line,
        character: location.ch,
      },
    }).then(
      /** @param {Location | Location[] | LocationLink[] | null} result */
      (result) => {
        this.emit("goTo", result);
      },
    );
  }

  /**
   * Request a link to the type definition of the current symbol. The results will not be displayed
   * unless they are within the same file URI
   * @param {IPosition} location
   */
  getTypeDefinition(location) {
    if (!this.isConnected || !this.isTypeDefinitionSupported()) {
      return;
    }

    this.sendRequest("textDocument/typeDefinition", {
      textDocument: {
        uri: this.documentInfo.documentUri,
      },
      position: {
        line: location.line,
        character: location.ch,
      },
    }).then(
      /** @param { Location | Location[] | LocationLink[] | null} result */
      (result) => {
        this.emit("goTo", result);
      },
    );
  }

  /**
   * Request a link to the implementation of the current symbol. The results will not be displayed
   * unless they are within the same file URI
   * @param {IPosition} location
   */
  getImplementation(location) {
    if (!this.isConnected || !this.isImplementationSupported()) {
      return;
    }

    this.sendRequest("textDocument/implementation", {
      textDocument: {
        uri: this.documentInfo.documentUri,
      },
      position: {
        line: location.line,
        character: location.ch,
      },
    }).then(
      /** @param {Location | Location[] | LocationLink[] | null} result */
      (result) => {
        this.emit("goTo", result);
      },
    );
  }

  /**
   * Request a link to all references to the current symbol. The results will not be displayed
   * unless they are within the same file URI
   * @param {IPosition} location
   */
  getReferences(location) {
    if (!this.isConnected || !this.isReferencesSupported()) {
      return;
    }

    this.sendRequest("textDocument/references", {
      textDocument: {
        uri: this.documentInfo.documentUri,
      },
      position: {
        line: location.line,
        character: location.ch,
      },
    }).then(
      /** @param {Location[] | null} result */
      (result) => {
        this.emit("goTo", result);
      },
    );
  }

  /**
   * The characters that trigger completion automatically.
   * @returns {string[]}
   */
  getLanguageCompletionCharacters() {
    if (!this.isConnected) {
      throw new Error(NOT_CONNECTED);
    }
    if (
      !(
        this.serverCapabilities &&
        this.serverCapabilities.completionProvider &&
        this.serverCapabilities.completionProvider.triggerCharacters
      )
    ) {
      return [];
    }
    return this.serverCapabilities.completionProvider.triggerCharacters;
  }

  /**
   * The characters that trigger signature help automatically.
   * @returns {string[]}
   */
  getLanguageSignatureCharacters() {
    if (!this.isConnected) {
      throw new Error(NOT_CONNECTED);
    }
    if (
      !(
        this.serverCapabilities &&
        this.serverCapabilities.signatureHelpProvider &&
        this.serverCapabilities.signatureHelpProvider.triggerCharacters
      )
    ) {
      return [];
    }
    return this.serverCapabilities.signatureHelpProvider.triggerCharacters;
  }

  /**
   * Does the server support go to definition?
   */
  isDefinitionSupported() {
    return !!(this.serverCapabilities &&
      this.serverCapabilities.definitionProvider);
  }

  /**
   * Does the server support go to type definition?
   */
  isTypeDefinitionSupported() {
    return !!(this.serverCapabilities &&
      this.serverCapabilities.typeDefinitionProvider);
  }

  /**
   * Does the server support go to implementation?
   */
  isImplementationSupported() {
    return !!(this.serverCapabilities &&
      this.serverCapabilities.implementationProvider);
  }

  /**
   * Does the server support find all references?
   */
  isReferencesSupported() {
    return !!(this.serverCapabilities &&
      this.serverCapabilities.referencesProvider);
  }

  close() {
    this._close = true;
  }
}

/** @param {number} timeMillis */
function sleepMillis(timeMillis) {
  return new Promise((resolve) => {
    setTimeout(resolve, timeMillis);
  });
}

const encoder = new TextEncoder();
const decoder = new TextDecoder();

/** @param {string} s */
function encodeString(s) {
  return encoder.encode(s);
}

/** @param {Uint8Array} a */
function decodeString(a) {
  return decoder.decode(a);
}

export default LspConnection;
