const {controlServerPort, token} = JSON.parse(Deno.args[0])

const handlers: Record<string, RequestHandler> = {
    echo: (requestPayload, sendResponse) => {
        sendResponse(requestPayload)
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

            handler?.(messageData.method, (resp) => {
                const responseMessage = {
                    id: messageData.id,
                    kind: 'response',
                    payload: resp,
                }
                ws.send(JSON.stringify(responseMessage))
            })
            return
        }
    }
}