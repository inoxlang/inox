//@ts-nocheck

/*
	This file contains component initialization logic and the following librairies:
	- Preact Signal library (MIT licensed): https://github.com/preactjs/signals
	- Surreal (MIT licensed) - https://github.com/gnat/surreal
	- CSS Scope Inline (MIT licensed) - https://github.com/gnat/css-scope-inline
*/

const INTERPOLATION_PATTERN = /\$\(([^)]*)\)/g

/**
 * @param {{
 *      signals: Record<string, unknown>
 * }} arg 
 */
function initComponent(arg) {
    const root = /** @type {HTMLElement} */(me())

    /** @type {WeakMap<Text, Interpolation[]>} */
    const textsWithInterpolations = new WeakMap() 

    walkNode(root, node => {
        if (node.nodeType != node.TEXT_NODE) {
            return
        }

        const textNode = /** @type {Text} */(node)
        let execArray = INTERPOLATION_PATTERN.exec(textNode.wholeText)

        if(execArray == null) {
            return
        }

        let interpolations = []

        while (execArray != null) {
            const rawInterpolation = execArray[1]
            interpolations.push(getInterpolation(rawInterpolation))

            execArray = INTERPOLATION_PATTERN.exec(textNode.wholeText)
        }

        textsWithInterpolations.set(textNode, interpolations)
    })
}

/** 
 *  @typedef Interpolation
 *  @property {string} name
 *  @property {[string]} default
 *  @property {[string]} type
 */

/** @param {string} rawInterpolation */
function getInterpolation(rawInterpolation) {

    /** @type {Interpolation} */
    const interpolation = {}
    let partIndex = 0;
    let partStart = 0;
    let inString = false

    loop:
    for(let i = 0; i < rawInterpolation.length; i++){
        switch(rawInterpolation[i]){
        case ':': case ')':
            if(inString){
                throw new Error(`invalid interpolation \`${rawInterpolation}\`: unterminated default value`)
            }
            switch(partIndex){
            case 0:
                interpolation.name = rawInterpolation.slice(partStart, i)
                partIndex++
                partStart = i+1
                break
            case 1:
                if(rawInterpolation[partStart] == '"'){
                    interpolation.default = rawInterpolation.slice(partStart+1, i-1)
                }

                partIndex++
                partStart = i+1
                break
            default:
                throw new Error(`invalid interpolation \`${rawInterpolation}\`: too many parts`)
            }

            continue loop
        case '"':
            if(partIndex == 0){
                throw new Error(`invalid interpolation \`${rawInterpolation}\`: the first part should be a name not a value, example: $(name:"default")`)
            }
            if(! inString){
                if(partIndex == 0 || partIndex > 1){
                    throw new Error(`invalid interpolation \`${rawInterpolation}\``)
                }

                inString = true
            } else {
                let backslashCount = 0
                for(let j = i-1; j >= 0 && rawInterpolation[j] == '\\'; j--){
                    backslashCount++
                }

                if(backslashCount % 2 == 1){
                    //escaped
                    continue
                }
                inString = false
            }
            break
        }
    }

    if(partIndex == 0){
        if(partStart == 0){
            interpolation.name = rawInterpolation.slice(partStart)
        }
    }
    return interpolation
}

/**
 * @param {Node} elem 
 * @param {(n: Node) => any} visit
 */
function walkNode(elem, visit) {
    visit(elem)
    for (const childNode of elem.childNodes) {
        walkNode(childNode, visit)
    }
}

// ------ Preact Signal library - transpiled to ES2022 ------
{
	//https://github.com/preactjs/signals/blob/a43821fa0f23846d86dd2e186b088e8f5c4f9d30/packages/core/src/index.ts

	function cycleDetected() {
		throw new Error("Cycle detected");
	}

	function mutationDetected() {
		throw new Error("Computed cannot have side-effects");
	}

	const identifier = Symbol.for("preact-signals");
	// Flags for Computed and Effect.
	const RUNNING = 1 << 0;
	const NOTIFIED = 1 << 1;
	const OUTDATED = 1 << 2;
	const DISPOSED = 1 << 3;
	const HAS_ERROR = 1 << 4;
	const TRACKING = 1 << 5;

	function startBatch() {
		batchDepth++;
	}

	function endBatch() {
		if (batchDepth > 1) {
			batchDepth--;
			return;
		}
		let error;
		let hasError = false;
		while (batchedEffect !== undefined) {
			let effect = batchedEffect;
			batchedEffect = undefined;
			batchIteration++;
			while (effect !== undefined) {
				const next = effect._nextBatchedEffect;
				effect._nextBatchedEffect = undefined;
				effect._flags &= ~NOTIFIED;
				if (!(effect._flags & DISPOSED) && needsToRecompute(effect)) {
					try {
						effect._callback();
					}
					catch (err) {
						if (!hasError) {
							error = err;
							hasError = true;
						}
					}
				}
				effect = next;
			}
		}
		batchIteration = 0;
		batchDepth--;
		if (hasError) {
			throw error;
		}
	}

	function batch(callback) {
		if (batchDepth > 0) {
			return callback();
		}
		/*@__INLINE__**/ startBatch();
		try {
			return callback();
		}
		finally {
			endBatch();
		}
	}

	// Currently evaluated computed or effect.
	let evalContext = undefined;
	let untrackedDepth = 0;

	function untracked(callback) {
		if (untrackedDepth > 0) {
			return callback();
		}
		const prevContext = evalContext;
		evalContext = undefined;
		untrackedDepth++;
		try {
			return callback();
		}
		finally {
			untrackedDepth--;
			evalContext = prevContext;
		}
	}

	// Effects collected into a batch.
	let batchedEffect = undefined;
	let batchDepth = 0;
	let batchIteration = 0;
	// A global version number for signals, used for fast-pathing repeated
	// computed.peek()/computed.value calls when nothing has changed globally.
	let globalVersion = 0;

	function addDependency(signal) {
		if (evalContext === undefined) {
			return undefined;
		}
		let node = signal._node;
		if (node === undefined || node._target !== evalContext) {
			/**
			 * `signal` is a new dependency. Create a new dependency node, and set it
			 * as the tail of the current context's dependency list. e.g:
			 *
			 * { A <-> B       }
			 *         ↑     ↑
			 *        tail  node (new)
			 *               ↓
			 * { A <-> B <-> C }
			 *               ↑
			 *              tail (evalContext._sources)
			 */
			node = {
				_version: 0,
				_source: signal,
				_prevSource: evalContext._sources,
				_nextSource: undefined,
				_target: evalContext,
				_prevTarget: undefined,
				_nextTarget: undefined,
				_rollbackNode: node,
			};
			if (evalContext._sources !== undefined) {
				evalContext._sources._nextSource = node;
			}
			evalContext._sources = node;
			signal._node = node;
			// Subscribe to change notifications from this dependency if we're in an effect
			// OR evaluating a computed signal that in turn has subscribers.
			if (evalContext._flags & TRACKING) {
				signal._subscribe(node);
			}
			return node;
		}
		else if (node._version === -1) {
			// `signal` is an existing dependency from a previous evaluation. Reuse it.
			node._version = 0;
			/**
			 * If `node` is not already the current tail of the dependency list (i.e.
			 * there is a next node in the list), then make the `node` the new tail. e.g:
			 *
			 * { A <-> B <-> C <-> D }
			 *         ↑           ↑
			 *        node   ┌─── tail (evalContext._sources)
			 *         └─────│─────┐
			 *               ↓     ↓
			 * { A <-> C <-> D <-> B }
			 *                     ↑
			 *                    tail (evalContext._sources)
			 */
			if (node._nextSource !== undefined) {
				node._nextSource._prevSource = node._prevSource;
				if (node._prevSource !== undefined) {
					node._prevSource._nextSource = node._nextSource;
				}
				node._prevSource = evalContext._sources;
				node._nextSource = undefined;
				evalContext._sources._nextSource = node;
				evalContext._sources = node;
			}
			// We can assume that the currently evaluated effect / computed signal is already
			// subscribed to change notifications from `signal` if needed.
			return node;
		}
		return undefined;
	}

	/** @internal */
	// @ts-ignore internal Signal is viewed as function
	function Signal(value) {
		this._value = value;
		this._version = 0;
		this._node = undefined;
		this._targets = undefined;
	}

	Signal.prototype.brand = identifier;

	Signal.prototype._refresh = function () {
		return true;
	};

	Signal.prototype._subscribe = function (node) {
		if (this._targets !== node && node._prevTarget === undefined) {
			node._nextTarget = this._targets;
			if (this._targets !== undefined) {
				this._targets._prevTarget = node;
			}
			this._targets = node;
		}
	};

	Signal.prototype._unsubscribe = function (node) {
		// Only run the unsubscribe step if the signal has any subscribers to begin with.
		if (this._targets !== undefined) {
			const prev = node._prevTarget;
			const next = node._nextTarget;
			if (prev !== undefined) {
				prev._nextTarget = next;
				node._prevTarget = undefined;
			}
			if (next !== undefined) {
				next._prevTarget = prev;
				node._nextTarget = undefined;
			}
			if (node === this._targets) {
				this._targets = next;
			}
		}
	};

	Signal.prototype.subscribe = function (fn) {
		const signal = this;
		return effect(function () {
			const value = signal.value;
			const flag = this._flags & TRACKING;
			this._flags &= ~TRACKING;
			try {
				fn(value);
			}
			finally {
				this._flags |= flag;
			}
		});
	};

	Signal.prototype.valueOf = function () {
		return this.value;
	};

	Signal.prototype.toString = function () {
		return this.value + "";
	};

	Signal.prototype.toJSON = function () {
		return this.value;
	};

	Signal.prototype.peek = function () {
		return this._value;
	};

	Object.defineProperty(Signal.prototype, "value", {
		get() {
			const node = addDependency(this);
			if (node !== undefined) {
				node._version = this._version;
			}
			return this._value;
		},
		set(value) {
			if (evalContext instanceof Computed) {
				mutationDetected();
			}
			if (value !== this._value) {
				if (batchIteration > 100) {
					cycleDetected();
				}
				this._value = value;
				this._version++;
				globalVersion++;
				/**@__INLINE__*/ startBatch();
				try {
					for (let node = this._targets; node !== undefined; node = node._nextTarget) {
						node._target._notify();
					}
				}
				finally {
					endBatch();
				}
			}
		},
	});

	function signal(value) {
		return new Signal(value);
	}

	function needsToRecompute(target) {
		// Check the dependencies for changed values. The dependency list is already
		// in order of use. Therefore if multiple dependencies have changed values, only
		// the first used dependency is re-evaluated at this point.
		for (let node = target._sources; node !== undefined; node = node._nextSource) {
			// If there's a new version of the dependency before or after refreshing,
			// or the dependency has something blocking it from refreshing at all (e.g. a
			// dependency cycle), then we need to recompute.
			if (node._source._version !== node._version ||
				!node._source._refresh() ||
				node._source._version !== node._version) {
				return true;
			}
		}
		// If none of the dependencies have changed values since last recompute then
		// there's no need to recompute.
		return false;
	}

	function prepareSources(target) {
		/**
		 * 1. Mark all current sources as re-usable nodes (version: -1)
		 * 2. Set a rollback node if the current node is being used in a different context
		 * 3. Point 'target._sources' to the tail of the doubly-linked list, e.g:
		 *
		 *    { undefined <- A <-> B <-> C -> undefined }
		 *                   ↑           ↑
		 *                   │           └──────┐
		 * target._sources = A; (node is head)  │
		 *                   ↓                  │
		 * target._sources = C; (node is tail) ─┘
		 */
		for (let node = target._sources; node !== undefined; node = node._nextSource) {
			const rollbackNode = node._source._node;
			if (rollbackNode !== undefined) {
				node._rollbackNode = rollbackNode;
			}
			node._source._node = node;
			node._version = -1;
			if (node._nextSource === undefined) {
				target._sources = node;
				break;
			}
		}
	}

	function cleanupSources(target) {
		let node = target._sources;
		let head = undefined;
		/**
		 * At this point 'target._sources' points to the tail of the doubly-linked list.
		 * It contains all existing sources + new sources in order of use.
		 * Iterate backwards until we find the head node while dropping old dependencies.
		 */
		while (node !== undefined) {
			const prev = node._prevSource;
			/**
			 * The node was not re-used, unsubscribe from its change notifications and remove itself
			 * from the doubly-linked list. e.g:
			 *
			 * { A <-> B <-> C }
			 *         ↓
			 *    { A <-> C }
			 */
			if (node._version === -1) {
				node._source._unsubscribe(node);
				if (prev !== undefined) {
					prev._nextSource = node._nextSource;
				}
				if (node._nextSource !== undefined) {
					node._nextSource._prevSource = prev;
				}
			}
			else {
				/**
				 * The new head is the last node seen which wasn't removed/unsubscribed
				 * from the doubly-linked list. e.g:
				 *
				 * { A <-> B <-> C }
				 *   ↑     ↑     ↑
				 *   │     │     └ head = node
				 *   │     └ head = node
				 *   └ head = node
				 */
				head = node;
			}
			node._source._node = node._rollbackNode;
			if (node._rollbackNode !== undefined) {
				node._rollbackNode = undefined;
			}
			node = prev;
		}
		target._sources = head;
	}

	function Computed(compute) {
		Signal.call(this, undefined);
		this._compute = compute;
		this._sources = undefined;
		this._globalVersion = globalVersion - 1;
		this._flags = OUTDATED;
	}

	Computed.prototype = new Signal();

	Computed.prototype._refresh = function () {
		this._flags &= ~NOTIFIED;
		if (this._flags & RUNNING) {
			return false;
		}
		// If this computed signal has subscribed to updates from its dependencies
		// (TRACKING flag set) and none of them have notified about changes (OUTDATED
		// flag not set), then the computed value can't have changed.
		if ((this._flags & (OUTDATED | TRACKING)) === TRACKING) {
			return true;
		}
		this._flags &= ~OUTDATED;
		if (this._globalVersion === globalVersion) {
			return true;
		}
		this._globalVersion = globalVersion;
		// Mark this computed signal running before checking the dependencies for value
		// changes, so that the RUNNING flag can be used to notice cyclical dependencies.
		this._flags |= RUNNING;
		if (this._version > 0 && !needsToRecompute(this)) {
			this._flags &= ~RUNNING;
			return true;
		}
		const prevContext = evalContext;
		try {
			prepareSources(this);
			evalContext = this;
			const value = this._compute();
			if (this._flags & HAS_ERROR ||
				this._value !== value ||
				this._version === 0) {
				this._value = value;
				this._flags &= ~HAS_ERROR;
				this._version++;
			}
		}
		catch (err) {
			this._value = err;
			this._flags |= HAS_ERROR;
			this._version++;
		}
		evalContext = prevContext;
		cleanupSources(this);
		this._flags &= ~RUNNING;
		return true;
	};

	Computed.prototype._subscribe = function (node) {
		if (this._targets === undefined) {
			this._flags |= OUTDATED | TRACKING;
			// A computed signal subscribes lazily to its dependencies when the it
			// gets its first subscriber.
			for (let node = this._sources; node !== undefined; node = node._nextSource) {
				node._source._subscribe(node);
			}
		}
		Signal.prototype._subscribe.call(this, node);
	};

	Computed.prototype._unsubscribe = function (node) {
		// Only run the unsubscribe step if the computed signal has any subscribers.
		if (this._targets !== undefined) {
			Signal.prototype._unsubscribe.call(this, node);
			// Computed signal unsubscribes from its dependencies when it loses its last subscriber.
			// This makes it possible for unreferences subgraphs of computed signals to get garbage collected.
			if (this._targets === undefined) {
				this._flags &= ~TRACKING;
				for (let node = this._sources; node !== undefined; node = node._nextSource) {
					node._source._unsubscribe(node);
				}
			}
		}
	};

	Computed.prototype._notify = function () {
		if (!(this._flags & NOTIFIED)) {
			this._flags |= OUTDATED | NOTIFIED;
			for (let node = this._targets; node !== undefined; node = node._nextTarget) {
				node._target._notify();
			}
		}
	};

	Computed.prototype.peek = function () {
		if (!this._refresh()) {
			cycleDetected();
		}
		if (this._flags & HAS_ERROR) {
			throw this._value;
		}
		return this._value;
	};

	Object.defineProperty(Computed.prototype, "value", {
		get() {
			if (this._flags & RUNNING) {
				cycleDetected();
			}
			const node = addDependency(this);
			this._refresh();
			if (node !== undefined) {
				node._version = this._version;
			}
			if (this._flags & HAS_ERROR) {
				throw this._value;
			}
			return this._value;
		},
	});

	function computed(compute) {
		return new Computed(compute);
	}

	function cleanupEffect(effect) {
		const cleanup = effect._cleanup;
		effect._cleanup = undefined;
		if (typeof cleanup === "function") {
			/*@__INLINE__**/ startBatch();
			// Run cleanup functions always outside of any context.
			const prevContext = evalContext;
			evalContext = undefined;
			try {
				cleanup();
			}
			catch (err) {
				effect._flags &= ~RUNNING;
				effect._flags |= DISPOSED;
				disposeEffect(effect);
				throw err;
			}
			finally {
				evalContext = prevContext;
				endBatch();
			}
		}
	}

	function disposeEffect(effect) {
		for (let node = effect._sources; node !== undefined; node = node._nextSource) {
			node._source._unsubscribe(node);
		}
		effect._compute = undefined;
		effect._sources = undefined;
		cleanupEffect(effect);
	}

	function endEffect(prevContext) {
		if (evalContext !== this) {
			throw new Error("Out-of-order effect");
		}
		cleanupSources(this);
		evalContext = prevContext;
		this._flags &= ~RUNNING;
		if (this._flags & DISPOSED) {
			disposeEffect(this);
		}
		endBatch();
	}

	function Effect(compute) {
		this._compute = compute;
		this._cleanup = undefined;
		this._sources = undefined;
		this._nextBatchedEffect = undefined;
		this._flags = TRACKING;
	}

	Effect.prototype._callback = function () {
		const finish = this._start();
		try {
			if (this._flags & DISPOSED)
				return;
			if (this._compute === undefined)
				return;
			const cleanup = this._compute();
			if (typeof cleanup === "function") {
				this._cleanup = cleanup;
			}
		}
		finally {
			finish();
		}
	};

	Effect.prototype._start = function () {
		if (this._flags & RUNNING) {
			cycleDetected();
		}
		this._flags |= RUNNING;
		this._flags &= ~DISPOSED;
		cleanupEffect(this);
		prepareSources(this);
		/*@__INLINE__**/ startBatch();
		const prevContext = evalContext;
		evalContext = this;
		return endEffect.bind(this, prevContext);
	};

	Effect.prototype._notify = function () {
		if (!(this._flags & NOTIFIED)) {
			this._flags |= NOTIFIED;
			this._nextBatchedEffect = batchedEffect;
			batchedEffect = this;
		}
	};

	Effect.prototype._dispose = function () {
		this._flags |= DISPOSED;
		if (!(this._flags & RUNNING)) {
			disposeEffect(this);
		}
	};

	function effect(compute) {
		const effect = new Effect(compute);
		try {
			effect._callback();
		}
		catch (err) {
			effect._dispose();
			throw err;
		}
		// Return a bound function instead of a wrapper like `() => effect._dispose()`,
		// because bound functions seem to be just as fast and take up a lot less memory.
		return effect._dispose.bind(effect);
	}

}

// ------ Surreal  ------

{
	//https://github.com/gnat/surreal/blob/b19cf6dd0680c5ef8c91d809131420cf0fbc033f/surreal.js

	var $ = { // You can use a different name than "$", but you must change the reference in any plugins you use!
		$: this, // Convenience for core internals.
		sugars: {}, // Extra syntax sugar for plugins.

		// Table of contents and convenient call chaining sugar. For a familiar "jQuery like" syntax. 🙂
		// Check before adding new: https://youmightnotneedjquery.com/
		sugar(e) {
			if (e == null) { console.warn(`Cannot use "${e}". Missing a character?`) }

			// General
			e.run = (value) => { return $.run(e, value) }
			e.remove = () => { return $.remove(e) }

			// Classes and CSS.
			e.classAdd = (name) => { return $.classAdd(e, name) }
			e.class_add = e.add_class = e.addClass = e.classAdd // Aliases
			e.classRemove = (name) => { return $.classRemove(e, name) }
			e.class_remove = e.remove_class = e.removeClass = e.classRemove // Aliases
			e.classToggle = (name) => { return $.classToggle(e, name) }
			e.class_toggle = e.toggle_class = e.toggleClass = e.classToggle // Aliases
			e.styles = (value) => { return $.styles(e, value) }

			// Events.
			e.on = (name, fn) => { return $.on(e, name, fn) }
			e.off = (name, fn) => { return $.off(e, name, fn) }
			e.offAll = (name) => { return $.offAll(e, name) }
			e.off_all = e.offAll
			e.trigger = (name) => { return $.trigger(e, name) }
			e.halt = () => { return $.halt(e) }

			// Attributes.
			e.attribute = (name, value) => { return $.attribute(e, name, value) }
			e.attributes = e.attribute
			e.attr = e.attribute

			// Add all plugin sugar.
			$._e = e // Plugin access to "e" for chaining.
			for (const [key, value] of Object.entries(sugars)) {
				e[key] = value.bind($) //e[key] = value
			}

			return e
		},

		// Return single element. Selector not needed if used with inline <script> 🔥
		// If your query returns a collection, it will return the first element.
		// Example
		//	<div>
		//		Hello World!
		//		<script>me().style.color = 'red'</script>
		//	</div>
		me(selector = null, start = document, warning = true) {
			if (selector == null) return $.sugar(start.currentScript.parentElement) // Just local me() in <script>
			if (selector instanceof Event) return $.me(selector.target) // Events return event.target
			if (typeof selector == 'string' && isSelector(selector, start, warning)) return $.sugar(start.querySelector(selector)) // String selector.
			if ($.isNodeList(selector)) return $.me(selector[0]) // If we got a list, just take the first element.
			if ($.isNode(selector)) return $.sugar(selector) // Valid element.
			return null // Invalid.
		},

		// any() is me() but always returns array of elements. Requires selector.
		// Returns an Array of elements (so you can use methods like forEach/filter/map/reduce if you want).
		// Example: any('button')
		any(selector, start = document, warning = true) {
			if (selector == null) return $.sugar([start.currentScript.parentElement]) // Just local me() in <script>
			if (selector instanceof Event) return $.any(selector.target) // Events return event.target
			if (typeof selector == 'string' && isSelector(selector, start, true, warning)) return $.sugar(Array.from(start.querySelectorAll(selector))) // String selector.
			if ($.isNode(selector)) return $.sugar([selector]) // Single element. Convert to Array.
			if ($.isNodeList(selector)) return $.sugar(Array.from(selector)) // Valid NodeList or Array.
			return null // Invalid.
		},

		// Run any function on element(s)
		run(e, f) {
			if ($.isNodeList(e)) e.forEach(_ => { run(_, f) })
			if ($.isNode(e)) { f(e); }
			return e
		},

		// Remove element(s)
		remove(e) {
			if ($.isNodeList(e)) e.forEach(_ => { remove(_) })
			if ($.isNode(e)) e.parentNode.removeChild(e)
			return // Special, end of chain.
		},

		// Add class to element(s).
		classAdd(e, name) {
			if (e === null || e === []) return null
			if (typeof name !== 'string') return null
			if (name.charAt(0) === '.') name = name.substring(1)
			if ($.isNodeList(e)) e.forEach(_ => { $.classAdd(_, name) })
			if ($.isNode(e)) e.classList.add(name)
			return e
		},

		// Remove class from element(s).
		classRemove(e, name) {
			if (typeof name !== 'string') return null
			if (name.charAt(0) === '.') name = name.substring(1)
			if ($.isNodeList(e)) e.forEach(_ => { $.classRemove(_, name) })
			if ($.isNode(e)) e.classList.remove(name)
			return e
		},

		// Toggle class in element(s).
		classToggle(e, name) {
			if (typeof name !== 'string') return null
			if (name.charAt(0) === '.') name = name.substring(1)
			if ($.isNodeList(e)) e.forEach(_ => { $.classToggle(_, name) })
			if ($.isNode(e)) e.classList.toggle(name)
			return e
		},

		// Add inline style to element(s).
		// Can use string or object formats.
		// 	String format: "font-family: 'sans-serif'"
		// 	Object format; { fontFamily: 'sans-serif', backgroundColor: '#000' }
		styles(e, value) {
			if (typeof value === 'string') { // Format: "font-family: 'sans-serif'"
				if ($.isNodeList(e)) e.forEach(_ => { styles(_, value) })
				if ($.isNode(e)) { attribute(e, 'style', (attribute(e, 'style') == null ? '' : attribute(e, 'style') + '; ') + value) }
				return e
			}
			if (typeof value === 'object') { // Format: { fontFamily: 'sans-serif', backgroundColor: '#000' }
				if ($.isNodeList(e)) e.forEach(_ => { styles(_, value) })
				if ($.isNode(e)) { Object.assign(e.style, value) }
				return e
			}
		},

		// Add event listener to element(s).
		// Match with: if(!event.target.matches(".selector")) return;
		//	📚️ https://developer.mozilla.org/en-US/docs/Web/API/Event
		//	✂️ Vanilla: document.querySelector(".thing").addEventListener("click", (e) => { alert("clicked") }
		on(e, name, fn) {
			if (typeof name !== 'string') return null
			if ($.isNodeList(e)) e.forEach(_ => { on(_, name, fn) })
			if ($.isNode(e)) e.addEventListener(name, fn)
			return e
		},

		off(e, name, fn) {
			if (typeof name !== 'string') return null
			if ($.isNodeList(e)) e.forEach(_ => { off(_, name, fn) })
			if ($.isNode(e)) e.removeEventListener(name, fn)
			return e
		},

		offAll(e) {
			if ($.isNodeList(e)) e.forEach(_ => { offAll(_) })
			if ($.isNode(e)) e = e.cloneNode(true)
			return e
		},

		// Trigger event / dispatch event.
		// ✂️ Vanilla: Events Dispatch: document.querySelector(".thing").dispatchEvent(new Event('click'))
		trigger(e, name) {
			if ($.isNodeList(e)) e.forEach(_ => { trigger(_, name) })
			if ($.isNode(e)) {
				const event = new CustomEvent(name, { bubbles: true })
				e.dispatchEvent(event)
			}
			return e
		},

		// Halt event / prevent default.
		halt(e) {
			if (e instanceof Event) {
				if (!e.preventDefault) {
					e.returnValue = false
				} else {
					e.preventDefault()
				}
			}
			return e
		},

		// Add or remove attributes from element(s)
		attribute(e, name, value = undefined) {
			// Get. This one is special. Format: "name", "value"
			if (typeof name === 'string' && value === undefined) {
				if ($.isNodeList(e)) return [] // Not supported for Get. For many elements, wrap attribute() in any(...).run(...) or any(...).forEach(...)
				if ($.isNode(e)) return e.getAttribute(name)
				return null // No value.
			}
			// Remove.
			if (typeof name === 'string' && value === null) {
				if ($.isNodeList(e)) e.forEach(_ => { $.attribute(_, name, value) })
				e.removeAttribute(name)
				return e
			}
			// Add / Set.
			if (typeof name === 'string') {
				if ($.isNodeList(e)) e.forEach(_ => { $.attribute(_, name, value) })
				e.setAttribute(name, value)
				return e
			}
			// Format: { "name": "value", "blah": true }
			if (typeof name === 'object') {
				if ($.isNodeList(e)) e.forEach(_ => { Object.entries(name).forEach(([key, val]) => { attribute(_, key, val) }) })
				if ($.isNode(e)) Object.entries(name).forEach(([key, val]) => { attribute(e, key, val) })
				return e
			}
			return e
		},

		// Puts Surreal functions except for "restricted" in global scope.
		globalsAdd() {
			console.log(`Surreal: adding convenience globals to window`)
			restricted = ['$', 'sugar']
			for (const [key, value] of Object.entries(this)) {
				if (!restricted.includes(key)) window[key] != 'undefined' ? window[key] = value : console.warn(`Surreal: "${key}()" already exists on window. Skipping to prevent overwrite.`)
				window.document[key] = value
			}
		},

		// ⚙️ Used internally. Is this an element / node?
		isNode(e) {
			return (e instanceof HTMLElement || e instanceof SVGElement) ? true : false
		},

		// ⚙️ Used internally by DOM functions. Is this a list of elements / nodes?
		isNodeList(e) {
			return (e instanceof NodeList || Array.isArray(e)) ? true : false
		},

		// ⚙️ Used internally by DOM functions. Warning when selector is invalid. Likely missing a "#" or "."
		isSelector(selector = "", start = document, all = false, warning = true) {
			if (all && start.querySelectorAll(selector) == null) {
				if (warning) console.warn(`"${selector}" was not found. Missing a character? (. #)`)
				return false
			}
			if (start.querySelector(selector) == null) {
				if (warning) console.warn(`"${selector}" was not found. Missing a character? (. #)`)
				return false
			}
			return true // Valid.
		},
	}

	// 📦 Plugin: Effects
	var $effects = {
		// Fade out and remove element.
		// Equivalent to jQuery fadeOut(), but actually removes the element!
		fadeOut(e, fn = false, ms = 1000, remove = true) {
			thing = e

			if ($.isNodeList(e)) e.forEach(_ => { fadeOut(_, fn, ms) })
			if ($.isNode(e)) {
				(async () => {
					$.styles(e, 'max-height: 100%; overflow: hidden')
					$.styles(e, `transition: all ${ms}ms ease-out`)
					await tick()
					$.styles(e, 'max-height: 0%; padding: 0; opacity: 0')
					await sleep(ms, e)
					if (fn === 'function') fn()
					if (remove) $.remove(thing) // Remove element after animation is completed?
				})()
			}
		},
		fadeIn(e, fn = false, ms = 1000) {
			thing = e
			if ($.isNodeList(e)) e.forEach(_ => { fadeIn(_, fn, ms) })
			if ($.isNode(e)) {
				(async () => {
					$.styles(e, 'max-height: 100%; overflow: hidden')
					$.styles(e, `transition: all ${ms}ms ease-in`)
					await tick()
					$.styles(e, 'max-height: 100%; opacity: 1')
					await sleep(ms, e)
					if (fn === 'function') fn()
				})()
			}
		},
		$effects
	}
	$ = { ...$, ...$effects }
	$.sugars['fadeOut'] = (fn, ms) => { return $.fadeOut($._e, fn = false, ms = 1000) }
	$.sugars['fadeIn'] = (fn, ms) => { return $.fadeIn($._e, fn = false, ms = 1000) }
	$.sugars['fade_out', 'fade_in'] = $.sugars['fadeOut', 'fadeIn']

	$.globalsAdd() // Full convenience.

	console.log("Loaded Surreal.")

	// 🌐 Optional global helpers.
	const createElement = document.createElement.bind(document)
	const create_element = createElement
	const rAF = typeof requestAnimationFrame !== 'undefined' && requestAnimationFrame
	const rIC = typeof requestIdleCallback !== 'undefined' && requestIdleCallback
	// Sleep without async!
	function sleep(ms, e) {
		return new Promise(resolve => setTimeout(() => { resolve(e) }, ms))
	}
	// Wait for next animation frame.
	async function tick() {
		await new Promise(resolve => { requestAnimationFrame(resolve) })
	}
	// Loading helper. Why? So you don't overwrite window.onload. And predictable sequential loading!
	// <script>onloadAdd(() => { console.log("Page was loaded!") })</script>
	// <script>onloadAdd(() => { console.log("Lets do another thing!") })</script>
	function onloadAdd(f) {
		// window.onload was not set yet.
		if (typeof window.onload != 'function') {
			window.onload = f
			return
		}
		// If onload already is set, queue them together. This creates a sequential call chain as we add more functions.
		let onload_old = window.onload
		window.onload = () => {
			onload_old()
			f()
		}
	}
	const onload_add = add_onload = addOnload = onloadAdd // Aliases
}

// ------ CSS Scope Inline ------
{
	//https://github.com/gnat/css-scope-inline/blob/582e9709ec2902f2546bd560dd4b33410ccfe622/script.js

	window.cssScopeCount ??= 1 // Let extra copies share the scope count.
	window.cssScope ??= new MutationObserver(mutations => { // Allow 1 observer.
		document?.body?.querySelectorAll('style:not([ready])').forEach(node => { // Faster than walking MutationObserver results when recieving subtree (DOM swap, htmx, ajax, jquery).
			var scope = 'me__' + (window.cssScopeCount++) // Ready. Make unique scope, example: .me__1234
			node.parentNode.classList.add(scope)
			node.textContent = node.textContent
				.replace(/(?:^|\.|(\s|[^a-zA-Z0-9\-\_]))(me|this|self)(?![a-zA-Z])/g, '$1.' + scope) // Can use: me this self
				.replace(/((@keyframes|animation:|animation-name:)[^{};]*)\.me__/g, '$1me__') // Optional. Removes need to escape names, ex: "\.me"
				.replace(/(?:@media)\s(xs-|sm-|md-|lg-|xl-|sm|md|lg|xl|xx)/g, // Optional. Responsive design. Mobile First (above breakpoint): 🟢 None sm md lg xl xx 🏁  Desktop First (below breakpoint): 🏁 xs- sm- md- lg- xl- None 🟢 *- matches must be first!
					(match, part1) => { return '@media ' + ({ 'sm': '(min-width: 640px)', 'md': '(min-width: 768px)', 'lg': '(min-width: 1024px)', 'xl': '(min-width: 1280px)', 'xx': '(min-width: 1536px)', 'xs-': '(max-width: 639px)', 'sm-': '(max-width: 767px)', 'md-': '(max-width: 1023px)', 'lg-': '(max-width: 1279px)', 'xl-': '(max-width: 1535px)' }[part1]) }
				)
			node.setAttribute('ready', '')
		})
	}).observe(document.documentElement, { childList: true, subtree: true })
}
