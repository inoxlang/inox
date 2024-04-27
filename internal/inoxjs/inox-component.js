/// <reference types="./preact-signals.d.ts" />

const INTERPOLATION_PATTERN = /\$\(([^)]*)\)/g
const SIGNAL_SETTLING_TIMEOUT_MILLIS = 100

/** @type {WeakMap<Signal, Dependent[]>} */
const signalsToDependents = new WeakMap()

/** @type {WeakMap<Text, Dependent>} */
const textsWithInterpolations = new WeakMap()


/** @type {WeakMap<Element, Record<string, Signal>>} */
const hyperscriptComponentRootsToSignals = new WeakMap();

(function(){
	const observer = new MutationObserver((mutations, observer) => {
		/** @type {Map<HTMLElement, Set<string>[]>} */
		const updatedAttributeNames = new Map()

		for(const mutation of mutations){
			switch(mutation.type){
			case 'attributes':
				const signals = hyperscriptComponentRootsToSignals.get(mutation.target)
				if(signals && (mutation.target instanceof HTMLElement)){
					let list = updatedAttributeNames.get(mutation.target)
					if(list === undefined){
						list = []
						updatedAttributeNames.set(mutation.target, list)
					}
					list.push(mutation.attributeName)
				}
				break
			case 'childList':
				//Initialize new Hyperscript components.
				mutation.addedNodes.forEach(node => {
					if(isComponentRootElement(node)){
						initComponent({
							element: node,
							isHyperscriptComponent: true
						})
					}
				})
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
 * @param {{
 *      signals?: Record<string, Signal>
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

		const textNode = /** @type {Text} */(node)
		let execArray = INTERPOLATION_PATTERN.exec(textNode.wholeText)

		if (execArray == null) {
			return
		}

		const interpolations = []

		while (execArray != null) {
			interpolations.push(getInterpolation(execArray[0], execArray[1], execArray.index, textNode))

			execArray = INTERPOLATION_PATTERN.exec(textNode.wholeText)
		}

		/** @type {TextNodeDependent} */
		const textDependent = {
			type: "text",
			node: textNode,
			interpolations: interpolations,
			rerender: makeRenderTextNode(textNode, interpolations)
		}

		textDependent.rerender(initialState)
		textsWithInterpolations.set(textNode, textDependent)

		for (const interp of interpolations) {
			const signal = signals[interp.name]
			if (signal) {
				let dependents = signalsToDependents.get(signal)
				if (dependents === undefined) {
					dependents = []
					signalsToDependents.set(signal, dependents)
				}
				dependents.push(textDependent)
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
					dependent.rerender(state)
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
		if(name.startsWith('data-')){
			state[name.slice('data-'.length)] = state[name]
		}
	}
}	

/** 
 *  @typedef Interpolation
 *  @property {Text} node
 *  @property {string} name
 *  @property {number} startIndex
 *  @property {number} endIndex
 *  @property {string} [default]
 *  @property {string} [type]
 */

/** 
 *  @typedef {TextNodeDependent} Dependent
 */

/** 
 *  @typedef TextNodeDependent
 *  @property {"text"} type
 *  @property {Text} node
 *  @property {Interpolation[]} interpolations
 *  @property {(state: State) => void} rerender
 */

/** 
 *  @typedef {Record<string, string>} State
 */


/** 
 * @param {string} rawInterpolationWithDelims 
 * @param {string} rawInterpolation 
 * @param {number} delimStartIndex
 * @param {Text} node
 * */
function getInterpolation(rawInterpolationWithDelims, rawInterpolation, delimStartIndex, node) {

	/** @type {Interpolation} */
	const interpolation = {
		name: "???",
		node: node,
		startIndex: delimStartIndex,
		endIndex: delimStartIndex + rawInterpolationWithDelims.length
	}
	let partIndex = 0;
	let partStart = 0;
	let inString = false

	loop:
	for (let i = 0; i < rawInterpolation.length; i++) {
		switch (rawInterpolation[i]) {
			case ':': case ')':
				if (inString) {
					throw new Error(`invalid interpolation \`${rawInterpolation}\`: unterminated default value`)
				}
				switch (partIndex) {
					case 0:
						interpolation.name = rawInterpolation.slice(partStart, i)
						partIndex++
						partStart = i + 1
						break
					case 1:
						if (rawInterpolation[partStart] == "'") {
							interpolation.default = rawInterpolation.slice(partStart + 1, i - 1)
						}

						partIndex++
						partStart = i + 1
						break
					default:
						throw new Error(`invalid interpolation \`${rawInterpolation}\`: too many parts`)
				}

				continue loop
			case "'":
				if (partIndex == 0) {
					throw new Error(`invalid interpolation \`${rawInterpolation}\`: the first part should be a name not a value, example: $(name:"default")`)
				}
				if (!inString) {
					if (partIndex == 0 || partIndex > 1) {
						throw new Error(`invalid interpolation \`${rawInterpolation}\``)
					}

					inString = true
				} else {
					let backslashCount = 0
					for (let j = i - 1; j >= 0 && rawInterpolation[j] == '\\'; j--) {
						backslashCount++
					}

					if (backslashCount % 2 == 1) {
						//escaped
						continue
					}
					inString = false
				}
				break
		}
	}

	if (partIndex == 0) {
		if (partStart == 0) {
			interpolation.name = rawInterpolation.slice(partStart)
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
	 */
	return (state) => {
		let string = ""
		for (const part of parts) {
			if (typeof part == 'string') {
				string += part
			} else {
				const interpolation = part
				if (interpolation.name in state) {
					string += state[interpolation.name].toString()
				} else {
					string += interpolation.default ?? "???"
				}
			}
		}
		node.data = string
	}
}

/**
 * @param {Node} elem 
 * @param {(n: Node) => any} visit
 */
function walkNode(elem, visit) {
	visit(elem)
	elem.childNodes.forEach(childNode => {
		walkNode(childNode, visit)
	})
}


/**
 * @param {Node} node 
 * @returns {asserts node is HTMLElement}
 */
function isComponentRootElement(node){
	return (node instanceof HTMLElement) && Array.from(node.classList).some(className => className[0].toUpperCase() == className[0])
}

/**
 * @param {HTMLElement} element 
 */
function getDeduplicatedAttributeNames(element){
	const attributeNames = element.getAttributeNames()
	return Array.from(new Set(attributeNames)) //remove duplicates
}

/**
 * @param {string} name 
 */
function signalNameFromAttrName(attrName){
	if(attrName.startsWith('data-')){
		return attrName.slice('data-'.length)
	}
	return attrName
}