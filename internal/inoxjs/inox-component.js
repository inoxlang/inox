/// <reference types="./preact-signals.d.ts" />


(function () {
	const CONDITIONAL_DISPLAY_ATTR_NAME = "x-if"
	const FOR_LOOP_ATTR_NAME = "x-for"
	const KEY_ATTR_NAME = "x-key"
	const DISABLE_SCRIPTING_ATTR_NAME = "data-disable-scripting"
	const HYPERSCRIPT_ATTR_NAME = "_"


	const INTERPOLATION_PATTERN = new RegExp('[(]{2}' + '((?:[^)]|\\)[^)])+)' + '[)]{2}', 'g')
	const LOOSE_HS_ELEM_VAR_NAME_PATTERN = /(:[a-zA-Z_][_a-zA-Z0-9]*)/g
	const LOOSE_HS_ATTR_NAME_PATTERN = /(@[a-zA-Z_][_a-zA-Z0-9-]*)/g
	const FOR_LOOP_ATTR_NAME_PATTERN =
		/^(?<elemVarName>:[a-zA-Z_][_a-zA-Z0-9]*)\s+in\s+(?<arraySignalName>[$:@]?[a-zA-Z_][-_a-zA-Z0-9]*?)(?:\s*|\s+index\s+(?<indexVarName>:[a-zA-Z_][_a-zA-Z0-9]*))$/;

	const SIGNAL_SETTLING_TIMEOUT_MILLIS = 100
	const FOR_LOOP_RESULT_ELEMENT_SYMBOL = Symbol()

	/** 
	 * @type {WeakMap<Signal, Dependent[]>}
	 * 
	 * The list of dependents should not be 'stored' using a WeakRef because (for now)
	 * only signalsToDependents references them.
	 * */
	const signalsToDependents = new WeakMap()

	/** @type {WeakMap<Element, WeakRef<Record<string, Signal>>>} */
	const hyperscriptComponentRootsToSignals = new WeakMap();

	/** @type {WeakMap<ForLoopDependent, ForLoopInternalState>} */
	const forLoopsInternalState = new WeakMap();

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
						if (!(mutation.target instanceof HTMLElement) || isInDisabledScriptingRegion(mutation.target)) {
							continue
						}
						const attributeName = /** @type {string} */ (mutation.attributeName)
						const signals = hyperscriptComponentRootsToSignals.get(mutation.target)?.deref()
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
							if (!isInDisabledScriptingRegion(node) &&
								(node instanceof HTMLElement) &&
								isComponentRootElement(node) &&
								!isRegisteredHyperscriptComponent(node) &&
								!node.hasAttribute(FOR_LOOP_ATTR_NAME)) {
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
				const signals = hyperscriptComponentRootsToSignals.get(component)?.deref()
				if (signals === undefined) {
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
	 *      isHyperscriptComponent?: boolean,
	 *      elementScopeSlice?: Record<string, unknown>
	 * }} arg 
	 */
	function initComponent(arg) {
		//@ts-ignore
		const componentRoot = arg.element ?? me()

		if (!isComponentRootElement(componentRoot)) {
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

			for (const [name, value] of Object.entries(arg.elementScopeSlice ?? {})) {
				elementScope[name] = value
			}

			for (const varName in elementScope) {
				const signalName = signalNameFromElemVarName(varName)
				signals[signalName] = signal(elementScope[varName])
			}

			observeElementScope(componentRoot, signals, elementScope)

			hyperscriptComponentRootsToSignals.set(componentRoot, new WeakRef(signals))
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

		//Register text interpolations.

		walkNode(componentRoot, node => {
			if ((node instanceof HTMLElement) && node.hasAttribute(FOR_LOOP_ATTR_NAME)) {
				return 'prune'
			}

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
				textInterpolations.push(getInterpolation(execArray[0], execArray[1], execArray.index, signals))

				execArray = INTERPOLATION_PATTERN.exec(textNode.wholeText)
			}

			/** @type {TextNodeDependent} */
			const textDependent = {
				type: "text",
				node: new WeakRef(textNode),
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
						if (!dependents.includes(textDependent)) {
							dependents.push(textDependent)
						}
					}
				}
			}
		})

		//Register attribute interpolations.

		for (const attribute of Array.from(componentRoot.attributes)) {
			let execArray = INTERPOLATION_PATTERN.exec(attribute.value)

			if (execArray == null) {
				continue
			}

			const textInterpolations = []

			while (execArray != null) {
				textInterpolations.push(getInterpolation(execArray[0], execArray[1], execArray.index, signals))

				execArray = INTERPOLATION_PATTERN.exec(attribute.value)
			}

			/** @type {AttributeDependent} */
			const attributeDependent = {
				type: "attribute",
				attribute: new WeakRef(attribute),
				interpolations: textInterpolations,
				refresh: makeRefreshAttrValue(attribute, textInterpolations)
			}

			attributeDependent.refresh(initialState, componentRoot)

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
						if (!dependents.includes(attributeDependent)) {
							dependents.push(attributeDependent)
						}
					}
				}
			}
		}

		//Register conditionally displayed elements.

		walkNode(componentRoot, node => {
			//Ignore descendants of template elements.
			if ((node instanceof HTMLElement) && node.hasAttribute(FOR_LOOP_ATTR_NAME)) {
				return 'prune'
			}

			//Ignore non-elements and elements without the attribute.
			if (!(node instanceof HTMLElement) || !node.hasAttribute(CONDITIONAL_DISPLAY_ATTR_NAME)) {
				return
			}

			//Ignore descendant components and their descendants.
			if (node != componentRoot && isComponentRootElement(node)) {
				return 'prune'
			}

			const expression = /** @type {string} */ (node.getAttribute(CONDITIONAL_DISPLAY_ATTR_NAME))
			if (expression.trim() == "") {
				return
			}

			/** @type {ConditionallyDisplayedDependent} */
			const conditionallyDisplayed = {
				type: "conditional-display",
				element: new WeakRef(node),
				conditionExpression: expression,
				inexactSignalList: estimateSignalsUsedInHyperscriptExpr(expression, signals)
			}

			//Initial conditional display.

			const result = evaluateHyperscript(conditionallyDisplayed.conditionExpression, componentRoot)
			if (!result) {
				const element = conditionallyDisplayed.element.deref()
				if (!element) {
					throw new Error('unreachable')
				}
				element.style.display = 'none';
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
					if (!dependents.includes(conditionallyDisplayed)) {
						dependents.push(conditionallyDisplayed)
					}
				}
			}
		})


		//Register 'for' loops.

		walkNode(componentRoot, node => {
			if (node == componentRoot || !(node instanceof HTMLElement) || node.parentElement === null) {
				return
			}

			const hasAttribute = node.hasAttribute(FOR_LOOP_ATTR_NAME)

			//Ignore descendant components and their descendants.
			if (!hasAttribute && node != componentRoot && isComponentRootElement(node)) {
				return 'prune'
			}

			if (!hasAttribute) {
				return
			}

			const attributeValue = /** @type {string} */ (node.getAttribute(FOR_LOOP_ATTR_NAME))

			const execArray = FOR_LOOP_ATTR_NAME_PATTERN.exec(attributeValue)
			if (execArray === null || execArray.groups === undefined) {
				console.error(
					`invalid value for ${FOR_LOOP_ATTR_NAME} attribute: \`${attributeValue}\`` +
					`\nValid examples: \`:elem in :list\`, \`:elem in :list index :index\``)
				return
			}

			if (!isComponentRootElement(node)) {
				console.error(node, 'a valid component name is missing in the class list')
				return 'prune'
			}

			const templateElement = node
			const elemVarName = execArray.groups['elemVarName']
			const arraySignalName = execArray.groups['arraySignalName']
			const indexVarName = /** @type {string|undefined} */ (execArray.groups['indexVarName']);

			/** @type {ForLoopDependent} */
			const forLoopDependent = {
				type: "for-loop",
				elemVarName: elemVarName,
				indexVarName: indexVarName,
				arraySignalName: arraySignalName,
				templateElement: new WeakRef(templateElement),
			}

			//Initial render.

			templateElement.style.display = 'none';
			evaluateXForLoop(forLoopDependent, templateElement, componentRoot, initialState)

			//Add the dependent to the mapping <signal> -> <dependents> 

			const signal = signals[arraySignalName]
			if (signal) {
				let dependents = signalsToDependents.get(signal)
				if (dependents === undefined) {
					dependents = []
					signalsToDependents.set(signal, dependents)
				}
				if (!dependents.includes(forLoopDependent)) {
					dependents.push(forLoopDependent)
				}
			}

			return 'prune'
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
						switch (dependent.type) {
							case 'text':
								dependent.rerender(state, componentRoot)
								break
							case 'attribute':
								dependent.refresh(state, componentRoot)
								break
							case 'conditional-display':
								const element = dependent.element.deref()
								if (!element) {
									continue
								}

								const result = evaluateHyperscript(dependent.conditionExpression, componentRoot)

								if (result) {
									element.style.display = ''
								} else {
									element.style.display = 'none';
								}
								break
							case 'for-loop':
								const templateElement = dependent.templateElement.deref()
								if (!templateElement) {
									continue
								}

								evaluateXForLoop(dependent, templateElement, componentRoot, state)
								break
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
	 *  @property {string} expression
	 *  @property {string[]} inexactSignalList
	 *  @property {number} startIndex
	 *  @property {number} endIndex
	 *  @property {string} [type]
	 */

	/** 
	 *  @typedef {TextNodeDependent | AttributeDependent | ConditionallyDisplayedDependent | ForLoopDependent} Dependent
	 */

	/** 
	 * A TextNodeDependent represents an HTML Text Node that contains one or more interpolations
	 * and that is therefore dependent on signals.
	 *  @typedef TextNodeDependent
	 *  @property {"text"} type
	 *  @property {WeakRef<Text>} node
	 *  @property {Interpolation[]} interpolations
	 *  @property {(state: State, componentRoot: HTMLElement) => void} rerender
	 */

	/** 
	 * An AttributeDependent represents an HTML attribute that contains one or more interpolations
	 * and that is therefore dependent on signals.
	 *  @typedef AttributeDependent
	 *  @property {"attribute"} type
	 *  @property {WeakRef<Attr>} attribute
	 *  @property {Interpolation[]} interpolations
	 *  @property {(state: State, componentRoot: HTMLElement) => void} refresh
	 */

	/** 
	 * A TextNodeDependent represents an HTML Node that is visible while a condition (Hyperscript expression)
	 * evaluates to true.
	 *  @typedef ConditionallyDisplayedDependent
	 *  @property {"conditional-display"} type
	 *  @property {WeakRef<HTMLElement>} element
	 *  @property {string} conditionExpression
	 *  @property {string[]} inexactSignalList
	 */

	/** 
	 * A ForLoopDependent a client-side for-loop that creates sibling elements from a template element.
	 *  @typedef ForLoopDependent
	 *  @property {"for-loop"} type
	 *  @property {WeakRef<HTMLElement>} templateElement
	 *  @property {string} arraySignalName
	 *  @property {string} elemVarName
	 *  @property {string|undefined} indexVarName
	 */

	/** 
	 *  @typedef {Record<string, string>} State
	 */


	/** 
	 *  @typedef {({lastRenderElements: Set<HTMLElement>, currentValue: unknown})} ForLoopInternalState
	 */

	/** 
	 * @param {string} rawInterpolationWithDelims 
	 * @param {string} rawInterpolation 
	 * @param {number} delimStartIndex
	 * @param {Record<string, Signal>} signals
	 * */
	function getInterpolation(rawInterpolationWithDelims, rawInterpolation, delimStartIndex, signals) {

		/** @type {Interpolation} */
		const interpolation = {
			expression: rawInterpolation,
			startIndex: delimStartIndex,
			endIndex: delimStartIndex + rawInterpolationWithDelims.length,
			inexactSignalList: estimateSignalsUsedInHyperscriptExpr(rawInterpolation, signals)
		}

		return interpolation
	}

	/**
	 * @param {string} template
	 * @param {Interpolation[]} interpolations
	 * 
	 */
	function getStringTemplateParts(template, interpolations) {
		let startPartIndex = 0

		/** @type {(string | Interpolation)[]} */
		const parts = []

		for (const interpolation of interpolations) {
			const before = template.slice(startPartIndex, interpolation.startIndex)
			if (before.length > 0) {
				parts.push(before)
			}
			parts.push(interpolation)
			startPartIndex = interpolation.endIndex
		}
		const after = template.slice(startPartIndex)
		if (after != "") {
			parts.push(after)
		}

		return parts
	}

	/**
	 * @param {Text} node
	 * @param {Interpolation[]} interpolations
	 */
	function makeRenderTextNode(node, interpolations) {
		const parts = getStringTemplateParts(node.wholeText, interpolations)

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
	* @param {Attr} attr
	* @param {Interpolation[]} interpolations
	*/
	function makeRefreshAttrValue(attr, interpolations) {
		const template = attr.value
		const parts = getStringTemplateParts(template, interpolations)
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
			attr.value = string
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
		if (!(node instanceof HTMLElement)) {
			return false
		}
		const firstClassName = node.classList.item(0)
		if (firstClassName === null) {
			return false
		}
		return (/[A-Z]/).test(firstClassName[0])
	}

	/**
	 * @param {Node} node 
	 * @returns {boolean}
	 */
	function isRegisteredHyperscriptComponent(node) {
		return hyperscriptComponentRootsToSignals.get(/** @type {any} */(node)) !== undefined
	}

	/**
	 * @param {Node} node
	 * @returns {boolean}
	 */
	function isInDisabledScriptingRegion(node) {
		if (node instanceof HTMLElement) {
			return node.closest("[" + DISABLE_SCRIPTING_ATTR_NAME + "]") !== null
		}
		return node.parentElement != null && isInDisabledScriptingRegion(node.parentElement)
	}


	/**
	 * @param {HTMLElement} element 
	 */
	function getDeduplicatedAttributeNames(element) {
		const attributeNames = element.getAttributeNames().filter(name => name != HYPERSCRIPT_ATTR_NAME)
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
	 * @param {ForLoopDependent} dependent
	 * @param {HTMLElement} templateElement
	 * @param {HTMLElement} componentRoot
	 * @param {State} state
	 */
	function evaluateXForLoop(dependent, templateElement, componentRoot, state) {
		const parentElement = templateElement.parentElement
		if (parentElement === null) {
			return
		}

		let internalState = forLoopsInternalState.get(dependent)
		if (internalState == undefined) {
			internalState = {
				lastRenderElements: new Set(),
				currentValue: undefined
			}
			forLoopsInternalState.set(dependent, internalState)
		}

		const array = evaluateHyperscript(dependent.arraySignalName, componentRoot)
		if (!Array.isArray(array)) {
			console.warn(templateElement, dependent.arraySignalName + ' did not evaluate to an Array but to ', array)
			return
		}

		//Note: comparing currentValue to array using the equality operator is not useful because the array can have the same identity.

		internalState.currentValue = array

		const keyTemplate = templateElement.getAttribute(KEY_ATTR_NAME)

		/**
		 * @param {number} index 
		 * @param {any} value 
		 */
		const createInstance = (index, value) => {
			const instanceElement = /** @type {HTMLElement} */ (templateElement.cloneNode(true))
			instanceElement.removeAttribute(DISABLE_SCRIPTING_ATTR_NAME)
			instanceElement.removeAttribute(FOR_LOOP_ATTR_NAME)
			instanceElement.style.display = '';

			//Make sure the element has a non-empty '_' attribute in order for the Hyperscript lib to initialize it.
			if (!instanceElement.getAttribute(HYPERSCRIPT_ATTR_NAME)) {
				instanceElement.setAttribute(HYPERSCRIPT_ATTR_NAME, "init")
			}
			return instanceElement
		}

		/**
		 * @param {HTMLElement} instance 
		 * @param {number} index 
		 * @param {unknown} value 
		 */
		function initInstance(instance, index, value) {
			//@ts-ignore
			_hyperscript.internals.runtime.initElement(instance)

			initComponent({
				element: instance,
				isHyperscriptComponent: true,
				elementScopeSlice: {
					[dependent.elemVarName]: value,
					...dependent.indexVarName ? {
						[dependent.indexVarName]: index,
					} : {}
				}
			})
		}

		if (keyTemplate === null) {

			for (const elem of Array.from(internalState.lastRenderElements)) {
				elem.remove()
			}
			internalState.lastRenderElements.clear()

			/** @type {HTMLElement|undefined} */
			let prevSibling;

			for (const [index, value] of array.entries()) {
				const instance = createInstance(index, value)
				internalState.lastRenderElements.add(instance)

				if (index == 0) {
					templateElement.insertAdjacentElement('afterend', instance)
				} else {
					prevSibling?.insertAdjacentElement('afterend', instance)
				}
				initInstance(instance, index, value)

				prevSibling = instance
			}
		} else {
			console.error(dependent.templateElement, 'keys are not supported yet')
		}
	}

	/**
	 * @param {HTMLElement} element 
	 * @returns {Record<string, unknown>}
	 */
	function getElementScope(element) {
		//@ts-ignore
		const internalData = _hyperscript.internals.runtime.getInternalData(element)
		if(internalData.elementScope === undefined){
			internalData.elementScope = {}
		}
		return internalData.elementScope
	}

	/**
	 * @param {HTMLElement} element 
	 * @param {Record<string, Signal>} signals
	 * @param {Record<string|symbol, unknown>} elementScope
	 */
	function observeElementScope(element, signals, elementScope) {
		//@ts-ignore
		const data = _hyperscript.internals.runtime.getInternalData(element)
		data.elementScope = new Proxy(elementScope, {
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
