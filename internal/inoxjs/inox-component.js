/// <reference types="./preact-signals.d.ts" />

(function () {
	const CONDITIONAL_DISPLAY_ATTR_NAME = "x-if"
	const INTERPOLATION_PATTERN = new RegExp('[(]{2}' + '((?:[^)]|\\)[^)])+)' + '[)]{2}', 'g')
	const LOOSE_HS_ELEM_VAR_NAME_PATTERN = /(:[a-zA-Z_][_a-zA-Z0-9]*)/g
	const LOOSE_HS_ATTR_NAME_PATTERN = /(@[a-zA-Z_][_a-zA-Z0-9-]*)/g


	const SIGNAL_SETTLING_TIMEOUT_MILLIS = 100

	/** @type {WeakMap<Signal, Dependent[]>} */
	const signalsToDependents = new WeakMap()

	/** @type {WeakMap<Element, Record<string, Signal>>} */
	const hyperscriptComponentRootsToSignals = new WeakMap();

	(function () {
		const observer = new MutationObserver((mutations, observer) => {
			/**
			 * Mapping <Hyperscript component root> -> list of relevant attribute names that have been updated. 
			 * @type {Map<HTMLElement, Set<string>>} 
			 * */
			const updatedAttributeNames = new Map()

			for (const mutation of mutations) {
				switch (mutation.type) {
					case 'attributes':
						if(! (mutation.target instanceof HTMLElement)){
							continue
						}
						const attributeName = /** @type {string} */ (mutation.attributeName)
						const signals = hyperscriptComponentRootsToSignals.get(mutation.target)
						if (signals) {
							let set = updatedAttributeNames.get(mutation.target)

							if (set === undefined) {
								//Create the Set of updated attributes for the component root.
								set = new Set()
								updatedAttributeNames.set(mutation.target, set)
							}

							//Add the attribute name if there is a corresponding signal.

							if (signalNameFromAttrName(attributeName) in signals) {
								set.add(attributeName)
							}
						}
						break
					case 'childList':
						mutation.addedNodes.forEach(node => {
							if (node instanceof HTMLElement && isComponentRootElement(node) && !isRegisteredHyperscriptComponent(node)) {
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

			for (const [component, attributeNames] of updatedAttributeNames) {
				const signals = hyperscriptComponentRootsToSignals.get(component)
				if(signals === undefined){
					throw new Error('unreachable')
				}
				batch(() => {
					for (const attrName of attributeNames) {
						const signalName = signalNameFromAttrName(attrName)
						const attribute =  /** @type {Attr} */ (component.attributes.getNamedItem(attrName))
						signals[signalName].value = attribute.value
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
	 * initComponent initializes an Inox component: it registers its signals, text interpolations and 
	 * conditionally displayed elements. Subtrees of descendant components are ignored.
	 * 
	 * @param {{
	 *      element?: HTMLElement
	 *      signals?: Record<string, Signal>
	 *      isHyperscriptComponent?: boolean
	 * }} arg 
	 */
	function initComponent(arg) {
		//@ts-ignore
		const componentRoot = arg.element ?? me()

		if(! isComponentRootElement(componentRoot)){
			console.error(componentRoot, 'is not a valid component root element, class list should start with a capitalized class name')
			return
		}

		//register signals

		const signals = arg.signals ?? {}

		if (arg.isHyperscriptComponent) {
			if (arg.signals) {
				throw new Error('signals should not be provided for an hyperscript components')
			}

			//Create a signal for each attribute.

			const attributeNames = getDeduplicatedAttributeNames(componentRoot)

			for (const attrName of attributeNames) {
				const signalName = signalNameFromAttrName(attrName)
				const attr = /** @type {Attr} */ (componentRoot.attributes.getNamedItem(attrName))
				signals[signalName] = signal(attr.value)
			}

			//Create a signal for each element variable.

			const elementScope = getElementScope(componentRoot)

			for (const varName in elementScope) {
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

		//register text interpolations

		walkNode(componentRoot, node => {
			if (node.nodeType != node.TEXT_NODE) {
				return
			}

			if (node != componentRoot && isComponentRootElement(node)) {
				return 'prune'
			}

			const textNode = /** @type {Text} */(node)
			let execArray = INTERPOLATION_PATTERN.exec(textNode.wholeText)

			if (execArray == null) {
				return
			}

			const textInterpolations = []

			while (execArray != null) {
				textInterpolations.push(getInterpolation(execArray[0], execArray[1], execArray.index, textNode, signals))

				execArray = INTERPOLATION_PATTERN.exec(textNode.wholeText)
			}

			/** @type {TextNodeDependent} */
			const textDependent = {
				type: "text",
				node: textNode,
				interpolations: textInterpolations,
				rerender: makeRenderTextNode(textNode, textInterpolations)
			}

			textDependent.rerender(initialState, componentRoot)

			//Add the dependent to the mapping <signal> -> <dependents> 

			for (const interp of textInterpolations) {
				for (const signalName of interp.inexactSignalList) {
					const signal = signals[signalName]
					if (signal) {
						let dependents = signalsToDependents.get(signal)
						if (dependents === undefined) {
							dependents = []
							signalsToDependents.set(signal, dependents)
						}
						if(! dependents.includes(textDependent)){
							dependents.push(textDependent)
						}
					}
				}
			}
		})

		//register conditionally displayed elements.

		walkNode(componentRoot, node => {
			if (!(node instanceof HTMLElement) || !node.hasAttribute(CONDITIONAL_DISPLAY_ATTR_NAME)) {
				return
			}

			if (node != componentRoot && isComponentRootElement(node)) {
				return 'prune'
			}

			const expression = /** @type {string} */ (node.getAttribute(CONDITIONAL_DISPLAY_ATTR_NAME))
			if(expression.trim() == ""){
				return
			}
			
			/** @type {ConditionallyDisplayedDependent} */
			const conditionallyDisplayed = {
				type: "conditional-display",
				element: node,
				conditionExpression: expression,
				inexactSignalList: estimateSignalsUsedInHyperscriptExpr(expression, signals)
			}

			//Initial conditional display.

			const result = evaluateHyperscript(conditionallyDisplayed.conditionExpression, componentRoot)
			if(!result){
				conditionallyDisplayed.element.style.display = 'none';
			}

			//Add the dependent to the mapping <signal> -> <dependents> 

			for (const signalName of conditionallyDisplayed.inexactSignalList) {
				const signal = signals[signalName]
				if (signal) {
					let dependents = signalsToDependents.get(signal)
					if (dependents === undefined) {
						dependents = []
						signalsToDependents.set(signal, dependents)
					}
					if(! dependents.includes(conditionallyDisplayed)){
						dependents.push(conditionallyDisplayed)
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
						switch(dependent.type){
						case 'text':
							dependent.rerender(state, componentRoot)
							break
						case 'conditional-display':
							const result = evaluateHyperscript(dependent.conditionExpression, componentRoot)
							if(result){
								dependent.element.style.display = ''
							} else {
								dependent.element.style.display = 'none';
							}
						}
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
	 *  @typedef {TextNodeDependent | ConditionallyDisplayedDependent} Dependent
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
	 * A TextNodeDependent represents an HTML Node that is visible while a condition (Hyperscript expression)
	 * evaluates to true.
	 *  @typedef ConditionallyDisplayedDependent
	 *  @property {"conditional-display"} type
	 *  @property {HTMLElement} element
	 *  @property {string} conditionExpression
	 *  @property {string[]} inexactSignalList
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
			inexactSignalList: estimateSignalsUsedInHyperscriptExpr(rawInterpolation, signals)
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
					if (result === undefined || result === null) {
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
	 *  @param {string} expression
	 *  @param {Record<string, Signal>} signals
	 *  @returns {string[]}
	 */
	function estimateSignalsUsedInHyperscriptExpr(expression, signals) {
		/** @type {string[]} */
		const list = []

		//Add element variables to the signal list.

		let execArray = LOOSE_HS_ELEM_VAR_NAME_PATTERN.exec(expression)

		while (execArray != null) {
			const name = signalNameFromElemVarName(execArray[0])
			if (name in signals) {
				list.push(name)
			}
			execArray = LOOSE_HS_ELEM_VAR_NAME_PATTERN.exec(expression)
		}

		//Add attribute names to the signal list.

		execArray = LOOSE_HS_ATTR_NAME_PATTERN.exec(expression)

		while (execArray != null) {
			const name = signalNameFromAttrName(execArray[0])
			if (name in signals) {
				list.push(name)
			}
			execArray = LOOSE_HS_ATTR_NAME_PATTERN.exec(expression)
		}

		return list
	}

	/**
	 * @param {Node} elem 
	 * @param {(n: Node) => 'prune'|void} visit
	 */
	function walkNode(elem, visit) {
		switch (visit(elem)) {
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
	function isComponentRootElement(node) {
		if(!(node instanceof HTMLElement)) {
			return false
		} 
		const firstClassName = node.classList.item(0)
		if(firstClassName === null){
			return false
		}
		return (/[A-Z]/).test(firstClassName[0])
	}

	/**
	 * @param {Node} node 
	 * @returns {boolean}
	 */
	function isRegisteredHyperscriptComponent(node) {

		return hyperscriptComponentRootsToSignals.get(/** @type {any} */ (node)) !== undefined
	}

	/**
	 * @param {HTMLElement} element 
	 */
	function getDeduplicatedAttributeNames(element) {
		const attributeNames = element.getAttributeNames()
		return Array.from(new Set(attributeNames)) //remove duplicates
	}

	/**
	 * @param {string} attrName 
	 */
	function signalNameFromAttrName(attrName) {
		if (attrName.startsWith('@')) {
			return attrName
		}
		return '@' + attrName
	}

	/**
	 * @param {string} elemVarName 
	 */
	function signalNameFromElemVarName(elemVarName) {
		if (elemVarName.startsWith(':')) {
			return elemVarName
		}
		return ':' + elemVarName
	}

	/**
	 * @param {string} expr
	 * @param {HTMLElement} element 
	*/
	function evaluateHyperscript(expr, element) {
		
		const owner = element
		//@ts-ignore
		const ctx = _hyperscript.internals.runtime.makeContext(owner, {}, element, undefined)

		//@ts-ignore
		return _hyperscript.evaluate(expr, ctx)
	}

	/**
	 * @param {HTMLElement} element 
	 */
	function getElementScope(element) {
		//@ts-ignore
		return _hyperscript.internals.runtime.getInternalData(element)?.elementScope ?? {}
	}

	/**
	 * @param {HTMLElement} element 
	 * @param {Record<string, Signal>} signals
	 */
	function observeElementScope(element, signals) {
		//@ts-ignore
		const data = _hyperscript.internals.runtime.getInternalData(element)
		data.elementScope = new Proxy(getElementScope(element), {
			set(target, name, newValue) {
				if (typeof name == 'string') {
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
