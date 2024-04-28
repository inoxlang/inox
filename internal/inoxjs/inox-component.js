/// <reference types="./preact-signals.d.ts" />

(function(){

	const INTERPOLATION_PATTERN = new RegExp('[(]{2}'+ '((?:[^)]|\\)[^)])+)' + '[)]{2}', 'g')
	const LOOSE_HS_ELEM_VAR_NAME_PATTERN = /(:[a-zA-Z_][_a-zA-Z0-9]*)/g
	const LOOSE_HS_ATTR_NAME_PATTERN = /(@[a-zA-Z_][_a-zA-Z0-9-]*)/g

	const SIGNAL_SETTLING_TIMEOUT_MILLIS = 100
	
	/** @type {WeakMap<Signal, Dependent[]>} */
	const signalsToDependents = new WeakMap()
	
	/** @type {WeakMap<Text, Dependent>} */
	const textsWithInterpolations = new WeakMap()
	
	/** @type {WeakMap<Element, Record<string, Signal>>} */
	const hyperscriptComponentRootsToSignals = new WeakMap();
	
	(function(){
		const observer = new MutationObserver((mutations, observer) => {
			/**
			 * Mapping <Hyperscript component root> -> list of relevant attribute names that have been updated. 
			 * @type {Map<HTMLElement, Set<string>[]>} 
			 * */
			const updatedAttributeNames = new Map()
	
			for(const mutation of mutations){
				switch(mutation.type){
				case 'attributes':
					const signals = hyperscriptComponentRootsToSignals.get(mutation.target)
					if(signals && (mutation.target instanceof HTMLElement)){
						let list = updatedAttributeNames.get(mutation.target)
	
						if(list === undefined){
							//Create the list of updated attributes for the component root.
							list = []
							updatedAttributeNames.set(mutation.target, list)
						}
	
						//Add the attribute name to the list of it has a corresponding signal.
	
						if(signalNameFromAttrName(mutation.attributeName) in signals){
							list.push(mutation.attributeName)
						}
					}
					break
				case 'childList':
					mutation.addedNodes.forEach(node => {
						if(isComponentRootElement(node) && !isRegisteredHyperscriptComponent(node)){
							//Initialize new Hyperscript component.
							
							//TODO: check that the component has been created from trusted HTML (HTMX, ..)
							//or from trusted Hyperscript code.
	
							setTimeout(() => { //make sure all initial children are present.
								initComponent({
									element: node,
									isHyperscriptComponent: true
								})
							}, 0)
						}
					})
					//We don't register new text interpolations because this is not secure.
					break
				}
			}
	
			//Update signals with the new attribute values.
	
			for(const [component, attributeNames] of updatedAttributeNames){
				batch(() => {
					const signals = hyperscriptComponentRootsToSignals.get(component)
					for(const attrName of attributeNames){
						const signalName = signalNameFromAttrName(attrName)
						signals[signalName].value = component.attributes.getNamedItem(attrName).value
					}
				})
			}
		})	
	
		observer.observe(document.documentElement, {
			subtree: true,
			attributes: true,
			childList: true,
		})
	})();
	
	
	/**
	 * initComponent initializes an Inox component: it registers its signals and the text interpolations.
	 * Text interpolations of descendant components are not registered.
	 * 
	 * @param {{
	 * 		element?: HTMLElement
	 *      signals?: Record<string, Signal>
	 *      isHyperscriptComponent?: boolean
	 * }} arg 
	 */
	function initComponent(arg) {
		const componentRoot = arg.element ?? /** @type {HTMLElement} */(me())
	
		//register signals
	
		const signals = arg.signals ?? {}
	
		if(arg.isHyperscriptComponent){
			if(arg.signals){
				throw new Error('signals should not be provided for an hyperscript components')
			}
	
			//Create a signal for each attribute.
	
			const attributeNames = getDeduplicatedAttributeNames(componentRoot)
	
			for(const attrName of attributeNames){
				const signalName = signalNameFromAttrName(attrName)
				signals[signalName] = signal(componentRoot.attributes.getNamedItem(attrName).value)
			}
	
			//Create a signal for each element variable.

			const elementScope = getElementScope(componentRoot)

			for(const varName in elementScope){
				const signalName = signalNameFromElemVarName(varName)
				signals[signalName] = signal(elementScope[varName])

			}
			observeElementScope(componentRoot, signals)

			hyperscriptComponentRootsToSignals.set(componentRoot, signals)
		}
	
	
		const initialState = getState(signals)
		const updatedSignalCount = signal(0)
	
		for (const [name, signal] of Object.entries(signals)) {
			const dispose = signal.subscribe(() => {
				//dispose the subscription if the component is no longer part of the DOM.
				if (!componentRoot.isConnected) {
					dispose()
					return
				}
	
				updatedSignalCount.value++
			})
		}
	
		//register interpolations
	
		walkNode(componentRoot, node => {
			if (node.nodeType != node.TEXT_NODE) {
				return
			}
	
			if(node != componentRoot && isComponentRootElement(node)){
				return 'prune'
			}
	
			const textNode = /** @type {Text} */(node)
			let execArray = INTERPOLATION_PATTERN.exec(textNode.wholeText)
	
			if (execArray == null) {
				return
			}
	
			const interpolations = []
	
			while (execArray != null) {
				interpolations.push(getInterpolation(execArray[0], execArray[1], execArray.index, textNode, signals))
	
				execArray = INTERPOLATION_PATTERN.exec(textNode.wholeText)
			}
	
			/** @type {TextNodeDependent} */
			const textDependent = {
				type: "text",
				node: textNode,
				interpolations: interpolations,
				rerender: makeRenderTextNode(textNode, interpolations)
			}
	
			textDependent.rerender(initialState, componentRoot)
			textsWithInterpolations.set(textNode, textDependent)
	
			//Add the dependent to the mapping <signal> -> <dependents> 

			for (const interp of interpolations) {
				for(const signalName of interp.inexactSignalList){
					const signal = signals[signalName]
					if (signal) {
						let dependents = signalsToDependents.get(signal)
						if (dependents === undefined) {
							dependents = []
							signalsToDependents.set(signal, dependents)
						}
						dependents.push(textDependent)
					}
				}
			}
		})
	
		//rendering
	
		let rendering = false;
	
		/** @param {number} timeMillis */
		const sleep = (timeMillis) => {
			return new Promise((resolve) => {
				setTimeout(() => resolve(null), timeMillis)
			})
		}
	
		const dispose = updatedSignalCount.subscribe(async (signalCount) => {
			//dispose the subscription if the component is no longer part of the DOM.
			if (!componentRoot.isConnected) {
				dispose()
				return
			}
	
			if (signalCount <= 0) {
				return
			}
	
			if (rendering) {
				return true
			}
	
			const waitStart = Date.now()
			rendering = true
	
			wait_for_signal_settling: while (true) {
				if (Date.now() - waitStart > SIGNAL_SETTLING_TIMEOUT_MILLIS) {
					console.error('signals take too much to settle, abort render')
					rendering = false
					return
				}
	
				const newSignalCount = updatedSignalCount.peek()
				if (newSignalCount > signalCount) {
					signalCount = newSignalCount
					await sleep(0)
					continue wait_for_signal_settling
				}
				break
			}
	
			const state = getState(signals)
			const updatedDependents = new Set()
	
			try {
				for (const [_, signal] of Object.entries(signals)) {
					let dependents = signalsToDependents.get(signal) ?? []
					if (dependents.length == 0) {
						updatedSignalCount.value = 0
						continue
					}
	
					//remove already updated dependents
					//TODO: add signal priority.
					dependents = dependents.filter(d => !updatedDependents.has(d))
					dependents.forEach(d => {
						updatedDependents.add(d)
					})
					
					//rerender dependents
	
					for (const dependent of dependents) {
						dependent.rerender(state, componentRoot)
					}
				}
			} finally {
				rendering = false
				updatedSignalCount.value = 0
			}
		})
	
	}
	
	/**
	 * @param {Record<string, Signal>} signals 
	 */
	function getState(signals) {
		/** @type {State} */
		const state = {}
		for (const name in signals) {
			state[name] = signals[name].peek()
		}
		return state
	}	
	
	/** 
	 *  @typedef Interpolation
	 *  @property {Text} node
	 *  @property {string} expression
	 *  @property {string[]} inexactSignalList
	 *  @property {number} startIndex
	 *  @property {number} endIndex
	 *  @property {string} [type]
	 */
	
	/** 
	 *  @typedef {TextNodeDependent} Dependent
	 */
	
	/** 
	 * A TextNodeDependent represents an HTML Text Node that contains one or more interpolations
	 * and that is therefore dependent on signals.
	 *  @typedef TextNodeDependent
	 *  @property {"text"} type
	 *  @property {Text} node
	 *  @property {Interpolation[]} interpolations
	 *  @property {(state: State, componentRoot: HTMLElement) => void} rerender
	 */
	
	/** 
	 *  @typedef {Record<string, string>} State
	 */
	
	
	/** 
	 * @param {string} rawInterpolationWithDelims 
	 * @param {string} rawInterpolation 
	 * @param {number} delimStartIndex
	 * @param {Text} node
	 * @param {Record<string, Signal>} signals
	 * */
	function getInterpolation(rawInterpolationWithDelims, rawInterpolation, delimStartIndex, node, signals) {
	
		/** @type {Interpolation} */
		const interpolation = {
			expression: rawInterpolation,
			node: node,
			startIndex: delimStartIndex,
			endIndex: delimStartIndex + rawInterpolationWithDelims.length,
			inexactSignalList: []
		}


        //Add element variables to the signal list.

        {
            let execArray = LOOSE_HS_ELEM_VAR_NAME_PATTERN.exec(interpolation.expression)
	
            while (execArray != null) {
                const name = signalNameFromElemVarName(execArray[0])
                if(name in signals){
                    interpolation.inexactSignalList.push(name)
                }
                execArray = LOOSE_HS_ELEM_VAR_NAME_PATTERN.exec(interpolation.expression)
            }
        }

        //Add attribute names to the signal list.

        {
            let execArray = LOOSE_HS_ATTR_NAME_PATTERN.exec(interpolation.expression)
	
            while (execArray != null) {
                const name = signalNameFromAttrName(execArray[0])
                if(name in signals){
                    interpolation.inexactSignalList.push(name)
                }
                execArray = LOOSE_HS_ATTR_NAME_PATTERN.exec(interpolation.expression)
            }
        }
	
		return interpolation
	}
	
	/**
	 * @param {Text} node
	 * @param {Interpolation[]} interpolations
	 */
	function makeRenderTextNode(node, interpolations) {
		const initialText = node.wholeText
		let startPartIndex = 0
	
		/** @type {(string | Interpolation)[]} */
		const parts = []
	
		for (const interpolation of interpolations) {
			const before = initialText.slice(startPartIndex, interpolation.startIndex)
			if (before.length > 0) {
				parts.push(before)
			}
			parts.push(interpolation)
			startPartIndex = interpolation.endIndex
		}
		const after = initialText.slice(startPartIndex)
		if (after != "") {
			parts.push(after)
		}
	
		/**
		 * @param {State} state
		 * @param {HTMLElement} componentRoot
		 */
		return (state, componentRoot) => {
			let string = ""
			for (const part of parts) {
				if (typeof part == 'string') {
					string += part
				} else {
					const interpolation = part
					const result = evaluateHyperscript(interpolation.expression, componentRoot)
					if(result === undefined || result === null){
						string += '?'
					} else {
						string += result.toString()
					}
				}
			}
			node.data = string
		}
	}
	
	/**
	 * @param {Node} elem 
	 * @param {(n: Node) => 'prune'|void} visit
	 */
	function walkNode(elem, visit) {
		switch(visit(elem)){
		case 'prune':
			return
		}
		elem.childNodes.forEach(childNode => {
			walkNode(childNode, visit)
		})
	}
	
	
	/**
	 * @param {Node} node 
	 * @returns {boolean}
	 */
	function isComponentRootElement(node){
		return (node instanceof HTMLElement) && Array.from(node.classList).some(className => className[0].toUpperCase() == className[0])
	}
	
	/**
	 * @param {Node} node 
	 * @returns {boolean}
	 */
	function isRegisteredHyperscriptComponent(node){
		return hyperscriptComponentRootsToSignals.get(node) !== undefined
	}
	
	/**
	 * @param {HTMLElement} element 
	 */
	function getDeduplicatedAttributeNames(element){
		const attributeNames = element.getAttributeNames()
		return Array.from(new Set(attributeNames)) //remove duplicates
	}
	
	/**
	 * @param {string} attrName 
	 */
	function signalNameFromAttrName(attrName){
		return '@'+attrName
	}

	/**
	 * @param {string} elemVarName 
	 */
	function signalNameFromElemVarName(elemVarName){
		if(elemVarName.startsWith(':')){
			return elemVarName
		}
		return ':'+elemVarName
	}

	/**
	 * @param {string} expr
	 * @param {HTMLElement} element 
	 */
	function evaluateHyperscript(expr, element){
		const owner = element
		const ctx = _hyperscript.internals.runtime.makeContext(owner, {}, element, undefined)
		return _hyperscript.evaluate(expr, ctx)
	}

	/**
	 * @param {HTMLElement} element 
	 */
	function getElementScope(element){
		return _hyperscript.internals.runtime.getInternalData(element)?.elementScope ?? {}
	}

	/**
	 * @param {HTMLElement} element 
	 * @param {Record<string, Signal>} signals
	 */
	function observeElementScope(element, signals){
		const data =  _hyperscript.internals.runtime.getInternalData(element)
		data.elementScope = new Proxy(getElementScope(element), {
			set(target, name, newValue){
				if(typeof name == 'string'){
					setTimeout(() => { //wait for the end of hyperscript execution
						const signalName = signalNameFromElemVarName(name)
						signals[signalName].value = newValue
					})
				}
				target[name] = newValue
				return true
			}
		})
	}
}());
