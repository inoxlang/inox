/// <reference types="./preact-signals.d.ts" />

const INTERPOLATION_PATTERN = /\$\(([^)]*)\)/g


/** @type {WeakMap<Signal, Dependent[]>} */
const signalsToDependents = new WeakMap()


/** @type {WeakMap<Text, Dependent>} */
const textsWithInterpolations = new WeakMap()

/**
 * @param {{
 *      signals?: Record<string, Signal>
 * }} arg 
 */
function initComponent(arg) {
	const componentRoot = /** @type {HTMLElement} */(me())

	//register signals

	const signals = arg.signals ?? {}
	const initialState = getState(signals)

	for (const [name, signal] of Object.entries(signals)) {
		const dispose = signal.subscribe(() => {
			//dispose the subscription if the component is no longer part of the DOM.
			if (!componentRoot.isConnected) {
				dispose()
				return
			}

			const dependents = signalsToDependents.get(signal) ?? []
			if (dependents.length == 0) {
				return
			}

			//get state

			const state = getState(signals)
			
			//rerender dependents

			for (const dependent of dependents) {
				dependent.rerender(state)
			}
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

		let interpolations = []

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

}

/**
 * @param {Record<string, Signal>} signals 
 */
function getState(signals){
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
						if (rawInterpolation[partStart] == '"') {
							interpolation.default = rawInterpolation.slice(partStart + 1, i - 1)
						}

						partIndex++
						partStart = i + 1
						break
					default:
						throw new Error(`invalid interpolation \`${rawInterpolation}\`: too many parts`)
				}

				continue loop
			case '"':
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
