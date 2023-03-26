const 
    cursorUpdateWaitTimeMillis = 20,
    EDITOR_WRAPPER_ID = "#editor-wrapper",
    CODE_CHUNK_LINES_DOTCLASS = ".code-chunk__lines",
    CODE_ERROR_DOTCLASS = ".code-chunk__error",
    TOKEN_DOTCLASS = ".token",
    IN_ERROR_DOTCLASS = ".in-error",
    TOOLTIP_DOTCLASS = ".code-chunk__tooltip",
    VISIBLE_DOTCLASS = '.visible',
    LINE_ELEMENT_SELECTOR = 'li[data-span]',
    NEWLINE_TOKEN_TYPE = 'newline',
    TREE_ITEM_DOTCLASS = '.tree-item',
    TREE_ITEM_CHILDREN_DOTCLASS = '.tree-item__children',
    CURRENT_LINE_DOTCLASS = '.current'



/** @type {View} */
let editor;

/** @type {View} */
let explorer;

setTimeout(() => {
    setupTerminals()

    //create editor

    editor = createEditorView()

    let editorObserver = new MutationObserver(mutations => {
        if(! mutations.some(m => m.addedNodes.length > 0)){
            return
        }
        let tokens = getAllTokens()

        forEachToken(tokens, (token) => {
            if (getErrorsAtToken(token).length > 0) {
                token.classList.add(IN_ERROR_DOTCLASS.slice(1))
            }
        })
    })
    editorObserver.observe(editor.element, {childList: true})

    //create explorer

    explorer = createExplorerView()

    let alreadyListeningTreeItems = new WeakSet()

    let explorerObserver = new MutationObserver(mutations => {
       let treeItems = Array.from(explorer.element.querySelectorAll(TREE_ITEM_DOTCLASS))
        .filter(isHTMLElement)
        .filter(item => !alreadyListeningTreeItems.has(item))

       treeItems.forEach(item => {
            alreadyListeningTreeItems.add(item)
            let childrenUL = item.querySelector(':scope > ' + TREE_ITEM_CHILDREN_DOTCLASS)

            if(childrenUL){ // folder
                let ul = childrenUL;
                item.addEventListener('click', ev => {
                    ul.classList.toggle(VISIBLE_DOTCLASS.slice(1))
                    ev.stopImmediatePropagation()
                })
            }
       })
    })
    explorerObserver.observe(explorer.element, {childList: true, subtree: true})
}, 0)


function createEditorView(){
    return createView({
        url: new URL("/editor", location.href),
        id: "editor-wrapper",
        onload: view => {
            // register some event listeners for events only handled on the client side
            view.element.addEventListener('mouseover', ev => {
                if(ev.target === null){
                    return
                }
        
                let token = findTokenWithMouseEvent(ev)
                if(token === undefined){
                    return
                }
        
                let tooltipChildren = getErrorsAtToken(token)
                if(tooltipChildren.length > 0){
                    showTooltip(token, tooltipChildren)
                }
            })
        
            view.element.addEventListener('mouseout', ev => {
                if(ev.target === null){
                    return
                }
        
                let token = findTokenWithMouseEvent(ev)
                if(token === undefined){
                    return
                }
        
                hideTooltip()
            })
        },
        preHandlers:  {
            'keydown':  (event, data) => {
                let selection = getSelection()
                assertNotNull(selection)
                assertNotNull(selection.anchorNode)

                let {tokenElement, tokenText} = getTokenFromOneSelectionEnd(selection.anchorNode) 

                // the following data is stored in the event's local data for later use by post handlers. 
                let userData = {
                    startToken: tokenElement,
                    endToken: tokenElement,
                    rangeLength: 0,
                    position: 0
                }

                data.userData = userData;

                if (data.selection.range){ 
                    let rangeLength = getTextContentInCodeLinesRange(data.selection.range).length
                    userData.rangeLength = rangeLength

                    userData.startToken = getTokenFromOneSelectionEnd(data.selection.range.startContainer, {moveToNextIfNewline: true}).tokenElement
                    userData.endToken = getTokenFromOneSelectionEnd(data.selection.range.endContainer).tokenElement
                }

                {
                    if(data.selection.anchorElem == data.selection.focusElem){
                        let [start] = getSpan(data.selection.anchorElemData);
                        userData.position = start + Math.max(data.selection.anchorOffset, data.selection.focusOffset)
                    } else {
                        let [start] = getSpan(data.selection.focusElemData);
                        userData.position = start + data.selection.focusOffset
                    }
                }

                var line = tokenElement.closest(LINE_ELEMENT_SELECTOR)
                assertHTMLelement(line)

                switch (event.key) {
                    case 'ArrowLeft': case 'ArrowRight':
                        var range = new Range()
                        range.collapse(true);

                        if(event.key == 'ArrowRight' && selection.anchorOffset == tokenText.data.length){ // move cursor to next token
                            if(tokenElement.nextElementSibling == null){ // no next token -> move to next line
                                if (!line.nextElementSibling){ // no next line
                                    return 'handled'
                                }

                                let nextLine = line.nextElementSibling;
                                assertHTMLelement(nextLine)
                                let [prevLineStart] = getSpan(nextLine.dataset)

                                let tokens = getSortedTokensOfLine(nextLine);
                                setCursorAtToken(prevLineStart+1, tokens)
                            } else {
                                let child = tokenElement.nextElementSibling.firstChild;
                                assertTextNode(child)
        
                                range.setStart(child, 1);
                                selection.removeAllRanges()
                                selection.addRange(range)
                            }
                        
                            return 'handled'
                        } else if(event.key == 'ArrowLeft' && selection.anchorOffset == 0){ // move cursor to previous token
                            let prevToken = tokenElement.previousElementSibling;

                            if(prevToken == null){
                                range.setStart(selection.anchorNode, 0);
                                return 'handled'
                            }
                            assertHTMLelement(prevToken)

                            if(getTokenType(prevToken.dataset) == NEWLINE_TOKEN_TYPE) { // move to previous line
                                if (!line.previousElementSibling){ // no previous line
                                    return 'handled'
                                }
            
                                let prevLine = line.previousElementSibling;
                                assertHTMLelement(prevLine)
                                let [_, prevLineEnd] = getSpan(prevLine.dataset)

                                let tokens = getSortedTokensOfLine(prevLine);
                                setCursorAtToken(prevLineEnd, tokens)
                            } else {
                                let child = prevToken.firstChild;
                                assertTextNode(child)
                                range.setStart(child, child.data.length-1);
                                selection.removeAllRanges()
                                selection.addRange(range)
                            }
                            return 'handled'
                        }

                        return 'default'
                    case 'ArrowDown': {
                        var line = tokenElement.closest(LINE_ELEMENT_SELECTOR)
                        assertHTMLelement(line)

                        if (!line.nextElementSibling){ // no next line
                            return 'handled'
                        }

                        let nextLine = line.nextElementSibling;
                        assertHTMLelement(nextLine)
                        let [lineStart] = getSpan(line.dataset)
                        let [nextLineStart, nextLineEnd] = getSpan(nextLine.dataset)

                        let offset = userData.position - lineStart
                        let newPosition = Math.min(nextLineStart + offset, nextLineEnd)

                        if(line.parentElement && line == line.parentElement.firstElementChild) { //special case for first line
                            newPosition += 1
                        }
                        
                        let tokens = getSortedTokensOfLine(nextLine);
                        setCursorAtToken(newPosition, tokens)
                        return "handled"
                    }
                    case 'ArrowUp': {
                        if (!line.previousElementSibling){ // no previous line
                            return 'handled'
                        }

                        let prevLine = line.previousElementSibling;
                        assertHTMLelement(prevLine)
                        let [lineStart] = getSpan(line.dataset)
                        let [prevLineStart, prevLineEnd] = getSpan(prevLine.dataset)

                        let offset = userData.position - lineStart
                        let newPosition = Math.min(prevLineStart + offset, prevLineEnd)

                        if(prevLine.parentElement && prevLine == prevLine.parentElement.firstElementChild) { //special case for first line
                            newPosition -= 1
                        }
                        
                        let tokens = getSortedTokensOfLine(prevLine);
                        setCursorAtToken(newPosition, tokens)
                        return "handled"
                        
                    }
                    case 'Home': {
                        let [lineStart] = getSpan(line.dataset)
                        let newPosition = lineStart + 1; // we offset by one to ignore the newline token

                        let tokens = getSortedTokensOfLine(line);
                        setCursorAtToken(newPosition, tokens)
                        return "handled"
                    }
                    case 'End': {
                        let [_, lineEnd] = getSpan(line.dataset)
                        let newPosition = lineEnd;

                        let tokens = getSortedTokensOfLine(line);
                        setCursorAtToken(newPosition, tokens)
                        return "handled"
                    }
                }

                return "send-event"
            },
            'click': (event, data) => {
                Array.from(getLines()).forEach(line => {
                    line.classList.remove(CURRENT_LINE_DOTCLASS.slice(1))
                })
                getEditableUnordererdList()
                if(event.target && isHTMLElement(event.target) && event.target.closest(CODE_CHUNK_LINES_DOTCLASS) && event.target.tagName == 'LI'){
                    assertHTMLelement(event.target.parentElement)
                    event.target.classList.add(CURRENT_LINE_DOTCLASS.slice(1))
                } else {
                    let token = findTokenWithMouseEvent(event)
                    if(token && token.parentElement){
                        token.parentElement.classList.add(CURRENT_LINE_DOTCLASS.slice(1))
                    }
                }
                
                return 'send-event'
            },
            'cut': (event, data) => {
                // get text

                if(data.selectionData.range && event.clipboardData){
                    data.text = getTextContentInCodeLinesRange(data.selectionData.range)
                    event.clipboardData.setData('text', data.text)
                }

                return 'send-event'
            },
            'copy': (event, data) => {
                // get text

                if(data.selectionData.range && event.clipboardData){
                    data.text = getTextContentInCodeLinesRange(data.selectionData.range)
                    event.clipboardData.setData('text', data.text)
                }

                return 'handled'
            }
        },
        postHandlers: {
            'keydown': (event, data) => {
                let handleCtrl = false;
                if(event.key.length != 1) {
                    switch(event.key){
                    case 'Backspace': case 'Delete': case 'Enter': case 'Tab':
                        handleCtrl = event.key == 'Backspace';
                        break
                    default:
                        return 
                    }
                }
    
                if((!handleCtrl && event.ctrlKey) || event.metaKey){
                    return
                }

                setTimeout(() => {
                    let previousPosition = Number(data.userData?.position)
                    let line;
                    let newPosition = 0;

                    let rangeLength = Number(data.userData?.rangeLength)
                    if(isNaN(rangeLength)){
                        rangeLength = 0
                    }

                    switch (event.key){
                    case 'Backspace':
                        if(data.selection.range){
                            newPosition = previousPosition - rangeLength
                        } else if (event.ctrlKey) {
                            let [tokenStart, tokenEnd] = getSpan(data.selection.anchorElemData);
                            let tokenLength = tokenEnd - tokenStart
                            newPosition = previousPosition - tokenLength
                        } else {
                            newPosition = previousPosition - 1
                        }
                       
                        break
                    case 'Delete':
                        newPosition = previousPosition
                        break
                    case 'Enter':
                        newPosition = previousPosition + 1
                        break
                    case 'Tab':
                        newPosition = previousPosition + 4
                        break
                    default:
                        newPosition = previousPosition + 1
                        break
                    }
    
                   
                    line = findLineOfPosition(newPosition)
                    if (!line) {
                        return
                    }

                    let tokens = getSortedTokensOfLine(line)
    
                    setCursorAtToken(newPosition, tokens)
                }, cursorUpdateWaitTimeMillis)
            },
        }
    })
}



function getEditableUnordererdList(){
    let ul = editor.element.querySelector(CODE_CHUNK_LINES_DOTCLASS);
    assertHTMLelement(ul)
    return /** @type {HTMLElement} */ (ul)
}

function getLines(){
   return getEditableUnordererdList().children
}

/** @param {number} position */
function findLineOfPosition(position){
    let lines = getEditableUnordererdList()
    let lineList = Array.from(lines.children).filter(isHTMLElement)

    for(let line of lineList){
        let [lineStart, lineEnd] = getSpan(line.dataset)
        if(position >= lineStart && position <= lineEnd){
            return line;
        }
    }
}

/**
 * @param {number} spanStart 
 * @param {number} spanEnd 
 */
function findLinesOfSpan(spanStart, spanEnd){
    let lines = getEditableUnordererdList()
    let lineList = Array.from(lines.children).filter(isHTMLElement)


    let lineIndex = 0;
    let startLineIndex = -1;
    let endLineIndex = -1;

    // find first line in span
    while (lineIndex < lineList.length) {
        let line = lineList[lineIndex]

        let [lineStart, lineEnd] = getSpan(line.dataset)
        if(spanStart >= lineStart && spanStart <= lineEnd){
            startLineIndex = lineIndex
            break
        }

        lineIndex++;
    }
   
    if(startLineIndex < 0){
        return []
    }

    // find last line in span

    while (lineIndex < lineList.length) {
        let line = lineList[lineIndex]

        let [lineStart, lineEnd] = getSpan(line.dataset)
        if(spanEnd >= lineStart && spanEnd <= lineEnd){
            endLineIndex = lineIndex
            break
        }

        lineIndex++;
    }

    if(endLineIndex < 0){
        return lineList.slice(startLineIndex)
    }

    return lineList.slice(startLineIndex, endLineIndex+1)
}


/** 
 * @param {Node} node 
 * @param { {moveToNextIfNewline?: boolean}} options
 * */
function getTokenFromOneSelectionEnd(node, options = {}){
    /** @type {HTMLElement} */
    let tokenElement
    /** @type {Text} */
    let tokenText

    if(node instanceof Text){
        assertNotNull(node.parentElement)
        tokenText = node
        tokenElement = node.parentElement
    } else {
        assertHTMLelement(node)
        tokenElement = node
    }

    if (options.moveToNextIfNewline && getTokenType(tokenElement.dataset) == NEWLINE_TOKEN_TYPE){
        assertHTMLelement(tokenElement.nextElementSibling)
        tokenElement = tokenElement.nextElementSibling
    }

    assertTextNode(tokenElement.firstChild)
    tokenText = tokenElement.firstChild
    
    return {tokenElement, tokenText}
}

/** @param {DOMStringMap} data */
function getTokenType(data){
    return String(data.type)
}

/** @param {Element} line */
function getSortedTokensOfLine(line){
    let tokens = Array.from(line.querySelectorAll('[data-span]'))
    .filter(isHTMLElement)
    .sort((a, b) => {
        let [tokenStartA] = getSpan(a.dataset)
        let [tokenStartB] = getSpan(b.dataset)

        return tokenStartA - tokenStartB
    })

    return tokens
}


function getAllTokens(){
    let lines = getEditableUnordererdList()
    let lineList = Array.from(lines.children).filter(isHTMLElement)

    return lineList.map(line => getSortedTokensOfLine(line)).reduce((allTokens, lineTokens) => allTokens.concat(lineTokens), [])
}


/** @typedef {'break'} IterationControlAction */

/**
 * @param {Element[]} tokens 
 * @param { (token: HTMLElement, tokenStart: number, tokenEnd: number, tokenStartColumn: number, tokenEndColumn: number, type: string) 
 *  => (IterationControlAction|void)
 * } fn
 */
function forEachToken(tokens, fn){
    if (tokens.length == 0){
        return
    }
    let line = tokens[0].parentElement
    assertNotNull(line)

    let [lineStart] = getSpan(line.dataset);
    let trueLineStart = lineStart;

    if(line.parentElement && line.parentElement.firstElementChild == line){ //not first line
        trueLineStart++
    }

    loop: for(let token of tokens){
        assertHTMLelement(token)
        let [start, end] = getSpan(token.dataset)

        let action = fn(token, start, end, start-trueLineStart, end-trueLineStart, getTokenType(token.dataset))

        switch (action){
        case 'break':
            break loop;
        }
    }
}

/**
 * @param {number} newPosition 
 * @param {Element[]} tokens 
 */
function setCursorAtToken(newPosition, tokens){
    let tokenFound = false
    let newOffset = 0;

    forEachToken(tokens, (token, tokenStart, tokenEnd) => {
                    
        if(tokenFound || newPosition > tokenEnd){
            return
        }

        /** @type {ChildNode|null} */
        let child = token.firstChild
        if(child === null){ // empty token representing the start of line
            return
        }
        assertTextNode(child)

        newOffset = newPosition - tokenStart;

        //console.log("child", child, {newOffset, positon: newPosition, tokenStart, tokenEnd, len: child.data.length})

        if(newOffset > child.data.length){
            newOffset -= child.data.length;
            return
        }

        tokenFound = true;
        
        let editorWrapper = document.body.querySelector(EDITOR_WRAPPER_ID)
        assertHTMLelement(editorWrapper)

        let line = token.parentElement
        assertHTMLelement(line)
        let {bottom: editorBottom} = editorWrapper.getBoundingClientRect()
        let {height: lineHeight, top: lineTop, bottom: lineBottom} = line.getBoundingClientRect()

        if(lineTop - lineHeight < 0){
            editorWrapper.scrollTop += lineTop
        } else if(lineBottom + lineHeight > editorBottom) {
            editorWrapper.scrollTop += (lineBottom + lineHeight - editorBottom)
        } 

        updateCursorPosition(child, newOffset)
    })
}

/** 
 * @param {Node} child
 * @param {number} newOffset
 */
const updateCursorPosition = (child, newOffset) => {
    let selection = getSelection()
    assertNotNull(selection)
    var range = new Range()
    range.collapse(true);
    range.setStart(child, newOffset);

    selection.removeAllRanges()
    selection.addRange(range)
}


function setupTerminals(){
    let shellWrapper = document.getElementById('shell-wrapper')
    assertNotNull(shellWrapper)
    setupTerminal({
        endpoint: '/shell',
        element: shellWrapper,
        allowWrite: true
    })

    let outputWrapper = document.getElementById('program-output-wrapper')
    assertNotNull(outputWrapper)
    setupTerminal({
        endpoint: '/output',
        element: outputWrapper,
        allowWrite: false
    })
}


/**
 * @param { {
 *  element: HTMLElement
 *  endpoint: string,
 *  allowWrite?: boolean,
 * }} config
 */
function setupTerminal(config){
    //create the terminal

    // @ts-ignore
    let term = new Terminal({
        theme: {
            background: getComputedStyle(document.body).getPropertyValue('background'),
        }
    });

    //Object.defineProperty(window, "term", {value: term})
    term.open(config.element);

    // connect to backend

    // writing
    if (config.allowWrite){
        term.onData(/** @param {string} data */ data => {
            let binary = stringToBinary(data)
            patchBinaryData(config.endpoint, binary)
        })
        term.onBinary(/** @param {string} data */ data => {
            let binary = stringToBinary(data)
            patchBinaryData(config.endpoint, binary)
        })
    } else {
        term.onData(/** @param {string} data */ data => {
            if(data == '\r'){
                term.write('\n')
            }
        })
    }

    //reading
    var output = new EventSource(config.endpoint, {withCredentials: true})

    /**@param {Event} ev */
    output.onerror = function(ev){ 
        console.error(ev) 
    }

    /**@param {MessageEvent<any>} ev */
    output.onmessage = function(ev){
        let data = atob(ev.data) //decode base64
        term.write(data)
    }

    //logic for fitting the terminal, extracted from https://github.com/xtermjs/xterm.js/blob/master/addons/xterm-addon-fit/src/FitAddon.ts
    const MINIMUM_COLS = 2;
    const MINIMUM_ROWS = 1;
    const core = term._core;


    function proposeDimensions(){
        const dims = core._renderService.dimensions;

        if (dims.css.cell.width === 0 || dims.css.cell.height === 0) {
            return undefined;
        }

        const scrollbarWidth = term.options.scrollback === 0 ?
        0 : core.viewport.scrollBarWidth;

        const parentElementStyle = window.getComputedStyle(term.element.parentElement);
        const parentElementHeight = parseInt(parentElementStyle.getPropertyValue('height'));
        const parentElementWidth = Math.max(0, parseInt(parentElementStyle.getPropertyValue('width')));
        const elementStyle = window.getComputedStyle(term.element);
        const elementPadding = {
            top: parseInt(elementStyle.getPropertyValue('padding-top')),
            bottom: parseInt(elementStyle.getPropertyValue('padding-bottom')),
            right: parseInt(elementStyle.getPropertyValue('padding-right')),
            left: parseInt(elementStyle.getPropertyValue('padding-left'))
        };
        const elementPaddingVer = elementPadding.top + elementPadding.bottom;
        const elementPaddingHor = elementPadding.right + elementPadding.left;
        const availableHeight = parentElementHeight - elementPaddingVer;
        const availableWidth = parentElementWidth - elementPaddingHor - scrollbarWidth;
        const geometry = {
            cols: Math.max(MINIMUM_COLS, Math.floor(availableWidth / dims.css.cell.width)),
            rows: Math.max(MINIMUM_ROWS, Math.floor(availableHeight / dims.css.cell.height))
        };
        return geometry;
    }

    const dims = proposeDimensions();
    if (!dims || isNaN(dims.cols) || isNaN(dims.rows)) {
      return;
    }

    // Force a full render
    if (term.rows !== dims.rows || term.cols !== dims.cols) {
      core._renderService.clear();
      term.resize(dims.cols, dims.rows);
    }

}

/**
 * @param {string} s 
 */
function stringToBinary(s){
    let buffer = new Uint8Array(s.length);
    for (let i = 0; i < s.length; ++i) {
        buffer[i] = s.charCodeAt(i) & 255;
    }
    return buffer
}

/**
 * @param {DOMStringMap} dataset 
 */
function getSpan(dataset){
    let span = dataset.span;
    return  String(span).split(",").map(s => Number.parseInt(s))
}

/**
 * @param {DOMStringMap} dataset 
 */
function getLineColumn(dataset){
    let result = {
        line: Number.parseInt(String(dataset.line)),
        column: Number.parseInt(String(dataset.column)),
        ok: true,
    }

    if (isNaN(result.line) || isNaN(result.column)) {
        result.ok = false;
    }

    return result;
}


/** @param {HTMLElement} token */
function getErrorsAtToken(token) {
    let [tokenStart, tokenEnd] = getSpan(token.dataset)

    let errors = Array.from(editor.element.querySelectorAll(".code-chunk > "+CODE_ERROR_DOTCLASS))
        .filter(isHTMLElement)
        .filter(codeErr => {
            let {line: errorLine, column: errorColumn, ok} = getLineColumn(codeErr.dataset)
            if (!ok){
                return false
            }
            let [errorStart, errorEnd] = getSpan(codeErr.dataset)

            return tokenStart <= errorStart && errorStart <= tokenEnd
        })
    return errors
}


/** @param {MouseEvent} ev */
function findTokenWithMouseEvent(ev){
    assertHTMLelement(ev.target)

    if(ev.target.classList.contains(TOKEN_DOTCLASS.slice(1))) {
        return ev.target;
    } else {
        return getAncestorsElements(ev.target, {upTo: editor.element})
            .find(ancestor => ancestor.classList.contains(TOKEN_DOTCLASS.slice(1)))
    }
}

/** @param {Range} range */
function getTextContentInCodeLinesRange(range){
    let content = range.cloneContents()
    let lines = Array.from(content.querySelectorAll(LINE_ELEMENT_SELECTOR)).filter(isHTMLElement)
    return lines.map(l => l.textContent).join('\n')
}

/**
 * @param {HTMLElement} anchorElement 
 * @param {HTMLElement[]} elements 
 */
function showTooltip(anchorElement, elements){
    let elem = editor.element.querySelector(TOOLTIP_DOTCLASS)
    /** @type {HTMLElement} */
    let tooltipElem

    if( !(elem instanceof HTMLElement)) {
        tooltipElem = document.createElement('div')
        tooltipElem.classList.add(TOOLTIP_DOTCLASS.slice(1))
        editor.element.appendChild(tooltipElem)
    } else {
        tooltipElem = elem
        // if tooltip is already visible do nothing
        if(tooltipElem.classList.contains(VISIBLE_DOTCLASS.slice(1))){
            return
        }
    }

    //remove previous children
    tooltipElem.innerHTML = '';
    elements.forEach(e => tooltipElem.appendChild(e.cloneNode(true)))

    //show
    tooltipElem.classList.add(VISIBLE_DOTCLASS.slice(1))
    
    // move above the token
    let tokenTopY = anchorElement.getBoundingClientRect().top
    let newTopPosition = String(tokenTopY - 5 - tooltipElem.getBoundingClientRect().height) + "px"
    tooltipElem.style.top = newTopPosition
}

function hideTooltip(){
    let elem = editor.element.querySelector(TOOLTIP_DOTCLASS)
    if(elem instanceof HTMLElement){
        elem.classList.remove(VISIBLE_DOTCLASS.slice(1))
    }
}

function createExplorerView(){
    return createView({
        url: new URL("/explorer", location.href),
        id: "explorer-wrapper",
        onload: view => {},
    })
}