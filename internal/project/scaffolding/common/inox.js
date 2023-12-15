/// <reference types="./preact-signals.d.ts" />

const INTERPOLATION_PATTERN = /\$\(([^)]*)\)/g


/** @type {WeakMap<Signal, Dependent[]>} */
const signalsToDependents = new WeakMap()

//register interpolations

/** @type {WeakMap<Text, Interpolation[]>} */
const textsWithInterpolations = new WeakMap()

/**
 * @param {{
 *      signals?: Record<string, Signal>
 * }} arg 
 */
function initComponent(arg) {
	const componentRoot = /** @type {HTMLElement} */(me())

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
			const rawInterpolation = execArray[1]
			interpolations.push(getInterpolation(rawInterpolation, textNode))

			execArray = INTERPOLATION_PATTERN.exec(textNode.wholeText)
		}

		textsWithInterpolations.set(textNode, interpolations)
	})

	for (const [name, signal] of Object.entries(arg.signals ?? {})) {
		const dispose = signal.subscribe(() => {
			//dispose the subscription if the component is no longer part of the DOM.
			if (!componentRoot.isConnected) {
				dispose()
				return
			}

			const dependents = signalsToDependents.get(signal) ?? []
			for (const dependent of dependents) {
				if ('node' in dependent) {

				}
			}
		})
	}
}

/** 
 *  @typedef Interpolation
 *  @property {Text} node
 *  @property {string} name
 *  @property {string} [default]
 *  @property {string} [type]
 */

/** 
 *  @typedef {Interpolation} Dependent
 */


/** 
 * @param {string} rawInterpolation 
 * @param {Text} node
 * */
function getInterpolation(rawInterpolation, node) {

	/** @type {Interpolation} */
	const interpolation = {
		name: "???",
		node: node
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
 */
function rerenderInterpolation(node) {
	node.replaceWith()
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
