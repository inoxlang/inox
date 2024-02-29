/* <The _hyperscript parser is injected here. */

const {controlServerPort, token} = JSON.parse(Deno.args[0])

declare var parseHyperScript: (input: string) => unknown

const handlers: Record<string, RequestHandler> = {
    parseHyperScript: (requestPayload, sendResponse) => {
        const result = parseHyperScript(requestPayload as any)

        sendResponse(result)
    }
}


type RequestHandler = (request: unknown, sendResponse: (response: unknown) => void) => void

//Connection loop.
while (true) {
    const ws = new WebSocket('ws://localhost:'+controlServerPort+"?token="+token)
    let closePromise: Promise<void>|undefined;

    try {
        //Wait for the connection to be established.
        await new Promise<void>((resolve, reject) => {

            setTimeout(() => reject(), /*timeout*/100)

            ws.addEventListener('open', () => {
                closePromise = new Promise(resolve => {
                    ws.addEventListener('close', () => {
                        resolve()
                    })
                })
                resolve()
            })

            ws.addEventListener('message', ev => _handleMessage(ev, ws))
        })

        await closePromise;
    } catch {
        //The control server is not running.
        Deno.exit(1)
    }
}


function _handleMessage(ev: MessageEvent, ws: WebSocket){
    const messageData = JSON.parse(ev.data)
    switch (messageData.kind) {
        case 'request': {
            const handler = handlers[messageData.method]

            handler?.(messageData.payload, (resp) => {

                const responseMessage = {
                    id: messageData.id,
                    kind: 'response',
                    payload: resp,
                }

                try {
                    ws.send(JSON.stringify(responseMessage))
                } catch(reason) {
                    console.error(reason)
                }
            })
            return
        }
    }
}