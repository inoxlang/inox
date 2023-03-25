let eventForwardingElements = new WeakMap()

/**
 * @typedef View
 * @property {EventSource} eventSource
 * @property {HTMLElement} element
 * @property {URL} url
 * @property { {[key in keyof HTMLElementEventMap]?: PreHandler<key>} } preHandlers
 * @property { {[key in keyof HTMLElementEventMap]?: PostHandler<key>} } postHandlers
 */

/**
 * @template E
 * @callback PreHandler
 * @param {HTMLElementEventMap[E]} event
 * @param {LocalData<E>} additionalData
 * @returns {PreHandlerAction}
 */


/** @typedef {"send-event" | "handled" | "default"} PreHandlerAction */

/**
 * @template E
 * @callback PostHandler
 * @param {HTMLElementEventMap[E]} event
 * @param {LocalData<E>} localData
 * @returns {any}
 */

/**
 * @template {keyof HTMLElementEventMap} E
 * @typedef { (HTMLElementEventMap[E] extends KeyboardEvent ? LocalKeyboardEventData : 
 *   HTMLElementEventMap[E] extends ClipboardEvent ? LocalClipboardEventData : {}) } LocalData<E>
 */

/**
 * @typedef { {
*  anchorElem: HTMLElement,
*  focusElem: HTMLElement,
*  anchorElemData: DOMStringMap,
*  focusElemData: DOMStringMap,
*  anchorOffset: number,
*  focusOffset: number,
*  range?: Range,
* }} LocalSelectionData
*/


/**
 * @typedef { {
 *  selection: LocalSelectionData,
 *  userData?: Record<string, any>,
 * }} LocalKeyboardEventData
 */

/**
 * @typedef { {
*  text?: string,
*  selectionData: LocalSelectionData,
* }} LocalClipboardEventData
*/


/**
 * @param { {url: URL,
 *  id: string,
 *  onload: (view: View) => any,
 *  preHandlers?: {[key in keyof HTMLElementEventMap]?: PreHandler<key>}
 *  postHandlers?: {[key in keyof HTMLElementEventMap]?: PostHandler<key>}
 * }} config
 * @returns {View}
 */
function createView(config) {
    let {url, id, preHandlers, postHandlers} = config;

    let element = document.querySelector('#' + id)
    if(element === null) {
        throw new Error("element with id " + id + " does not exist")
    }
    if (!(element instanceof HTMLElement)) {
        throw new Error("element with id " + id + " is not an HTML element")
    }

    /** @type {View} */
    let view = {
        eventSource: new EventSource(url, {withCredentials: true}),
        element: element,
        url: url,
        preHandlers: preHandlers || {},
        postHandlers: postHandlers || {},
    }

    view.eventSource.onerror = ev => console.error(ev)

    view.eventSource.onmessage = ev => {
        view.element.innerHTML = ev.data
        registerEventListeners(view)
    }

    fetch(url).then(v => {
        v.text().then(html => {
            view.element.innerHTML = html
            registerEventListeners(view)
            config.onload(view)
        })
    }).catch(reason => console.error(reason))


    return view
}

/**
 * @param {View} view
 */
function registerEventListeners(view) {
    console.debug('register event listeners for', view.url.href)

    const elements = Array.from(view.element.querySelectorAll('[data-listened-events]'))
        .filter(isHTMLElement)
        .filter(e => !(eventForwardingElements.has(e)))

    
    let registeredEventTypes = new Set()

    for(const element of elements){
        eventForwardingElements.set(element, null)
        const eventTypes = String(element.dataset.listenedEvents).split(',')

        for (const eventType of eventTypes) {
            registeredEventTypes.add(eventType)
            registerEventListener(eventType, view, element, true)
        }
    }

    for(let eventType in view.preHandlers){
        if (! registeredEventTypes.has(eventType)){
            registerEventListener(eventType, view, view.element, false)
        }
    }


}

/**
 * 
 * @param {string} eventType 
 * @param {View} view 
 * @param {HTMLElement} element 
 * @param {boolean} sendToBackend
 */
function registerEventListener(eventType, view, element, sendToBackend){

    if (sendToBackend){
        console.debug('register', eventType,  'listener on', element, 'for', view.url.href);
    }

    element.addEventListener(eventType, ev => {
        if ((ev instanceof KeyboardEvent) && ev.code == 'AltRight') {
            return
        }

        let {sentData, localData} = extractEventData(ev);

        /**  @type {undefined|((...args: any[]) => PreHandlerAction)} */
        let preHandler = view.preHandlers[/** @type {keyof HTMLElementEventMap} */ (eventType)]

        /** @type {PreHandlerAction} */
        let action = 'send-event';
        if(preHandler){
            try {
                action = preHandler(ev, localData)
            } catch(err){
                console.error(err)
            }
        }

        if(action == 'handled'){
            ev.stopImmediatePropagation()
            ev.preventDefault()
            ev.returnValue = false
            return
        }

        if (action == 'default'){
            return   
        }

        ev.stopImmediatePropagation()
        ev.preventDefault()
        ev.returnValue = false

        if (sendToBackend){
            fetch(view.url, {
                method: 'PATCH',
                headers: {
                    'Content-Type': 'dom/event'
                },
                body: JSON.stringify(sentData),
            }).then(() => {
                /**  @type {Function|undefined} */
                let postHandler = view.postHandlers[/** @type {keyof HTMLElementEventMap} */ (eventType)]
                if(postHandler){
                    postHandler(ev, localData)
                }
            }).catch(err => {
                console.error(err)
            })
        }
    })
}

/**
 * @template {Event} E
 * @param {E} ev 
 * @returns { {
 *  sentData: object, 
 *  localData: 
 *      E extends KeyboardEvent ? LocalKeyboardEventData : 
 *      E extends ClipboardEvent ? LocalClipboardEventData :
 *      object 
 * } }
 */
function extractEventData(ev){
    if( !(ev.currentTarget instanceof HTMLElement) || !(ev.target instanceof HTMLElement)){
        return {localData: /** @type {any} */ ({}), sentData: {}}
    }

    let ancestor = ev.currentTarget.parentElement
    let ancestorData = []
    while (ancestor !== null && ancestor != document.body) {
        if (Object.keys(ancestor.dataset).length != 0) {
            ancestorData.unshift(ancestor.dataset)
        }
        ancestor = ancestor.parentElement
    }


    /** @type {Record<string, any>} */
    let sentData = {
        type: ev.type,
        forwarderClass: ev.currentTarget.className,
        forwarderData: ev.currentTarget.dataset,
        targetClass: ev.target.className,
        targetData: ev.target.dataset,
        ancestorData: ancestorData,
    }

    let localData = {}

    if (ev instanceof MouseEvent) {
        /** @type {(keyof MouseEvent)[]}*/
        let keys = ["x", "y", "screenX", "screenY", "pageX", "pageY", "clientX", "clientY", "offsetX", "offsetY", "button"]
        for(let key of keys) {
            sentData[key] = ev[key]
        }

        stringifyIntegerProperties(sentData, keys)
        localData = {...sentData}
    }

    if (ev instanceof ClipboardEvent) {
        /** @type {LocalClipboardEventData} */
        let localClipboardEventData = {
            selectionData: addSelectionData(sentData),
        }
        localData = localClipboardEventData

        if(ev.clipboardData){
            localClipboardEventData.text = ev.clipboardData.getData('text')
            sentData.text = localClipboardEventData.text
        }
    }

    if (ev instanceof KeyboardEvent) {
        /** @type {(keyof KeyboardEvent)[]}*/
        let keys = ["key", "ctrlKey", "altKey", "metaKey", "shiftKey"]
        for(let key of keys) {
            sentData[key] = ev[key]
        }
       
        let localSelectionData = addSelectionData(sentData)

        localData = /** @type {LocalKeyboardEventData} */ ({
            ...sentData,
            selection: localSelectionData,
        });
    }
  
    return {sentData: sentData, localData: /** @type {any} */ (localData)}
}


/**
 * @param {Record<string, any>} sentData 
 * @returns {LocalSelectionData}
 */
function addSelectionData(sentData){
    let selection = getSelection()
    assertNotNull(selection)

    /** @type {HTMLElement} */
    let anchorElem; {
        assertNotNull(selection.anchorNode)

        if (!isHTMLElement(selection.anchorNode)) {
            let parent = selection.anchorNode.parentElement
            assertNotNull(parent)
            anchorElem = parent
        } else {
            anchorElem = selection.anchorNode
        }
    }

    /** @type {HTMLElement} */
    let focusElem; {
        assertNotNull(selection.focusNode)

        if (!isHTMLElement(selection.focusNode)) {
            let parent = selection.focusNode.parentElement
            assertNotNull(parent)
            focusElem = parent
        } else {
            focusElem = selection.focusNode
        }
    }

    sentData.anchorElemData = anchorElem.dataset
    sentData.anchorOffset = String(selection.anchorOffset)
    
    sentData.focusElemData = focusElem.dataset
    sentData.focusOffset = String(selection.focusOffset)

    let range;
    if (selection.type == 'Range'){
        range = selection.getRangeAt(0) // multi range selection are not supported yet
    }

    return {
        anchorElemData: sentData.anchorElemData,
        focusElemData: sentData.focusElemData,
        anchorElem: anchorElem,
        focusElem: focusElem,
        anchorOffset: selection.anchorOffset,
        focusOffset: selection.focusOffset,
        range: range,
    }
}

/**
 * @param {any} e 
 * @returns {e is HTMLElement}
 */
function isHTMLElement(e) {
    return e instanceof HTMLElement
}

/**
 * @template T
 * @param {T} e 
 * @returns {asserts e is HTMLElement}
 */
function assertHTMLelement(e) {
    if(!isHTMLElement(e)){
        throw new Error("value should be an HTML element")
    }
}

/**
 * @template T
 * @param {T} e 
 * @returns {asserts e is Text}
 */
function assertTextNode(e) {
    if(! (e instanceof Text)){
        throw new Error("value should be a Text")
    }
}



/**
 * @template T
 * @param {T} e 
 * @returns {asserts e is NonNullable<T>}
 */
function assertNotNull(e) {
    if(e === null || e === undefined){
        throw new Error("value should not be null")
    }
}

/**
 * @param {Record<string, any>} data 
 * @param {string[]} keys 
 */
function stringifyIntegerProperties(data, keys){

    for(let k of Object.keys(data)){
        if (keys.some(key => key == k)){

            let val = data[k]
            if (!Number.isSafeInteger(val)) {
                throw new Error("value of property ." + k + " is not a safe integer")
            }
            data[k] = String(Number(val))
        }
    }
}

/**
 * @param {HTMLElement} element 
 */
function computeSelector(element){
    let ids = []
    if(element.id == "") {
        console.error(element)
        throw new Error("element has no id")
    }
    while (element) {
        ids.unshift(element.id)
        if (element.parentElement === null || element.parentElement.id == ""){
            break
        }
        element = element.parentElement
    }

    return ids.join(' > ')
}


/** 
 * @param {any} element 
 * @param { {upTo?: HTMLElement}} options
*/
function getAncestorsElements(element, options = {}){
    /** @type {HTMLElement[]} */
    var ancestors = [];
    while(element.parentElement != null && element.parentElement != options.upTo){
      ancestors.push(element.parentElement)
      element = element.parentElement;
    }
    return ancestors;
}

/**
 * @param {HTMLElement} element 
 * @param {HTMLElement} ancestor 
 */
function hasAncestor(element, ancestor){
    return getAncestorsElements(element).indexOf(ancestor) >= 0
}

/**
 * @param {string} url 
 * @param {BufferSource} data 
 */
function patchBinaryData(url, data){
    fetch(url, {
        method: 'PATCH',
        headers: {
            'Content-Type': 'application/octet-stream'
        },
        body: data,
    }).then(() => {
        //
    }).catch(err => {
        console.error(err)
    })

}



/**
 * @param {HTMLElement} element 
 * @param {HTMLElement} container 
 */
function isElementVisibleInContainer(element, container) {
    let { bottom: elemBotton, height: elemHeight, top: elemTop } = element.getBoundingClientRect()
    let containerRect = container.getBoundingClientRect()

    return elemTop <= containerRect.top ? 
        containerRect.top - elemTop <= elemHeight : 
        elemBotton - containerRect.bottom <= elemHeight
};