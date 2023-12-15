declare const identifier: unique symbol;
type Node = {
    _source: Signal;
    _prevSource?: Node;
    _nextSource?: Node;
    _target: Computed | Effect;
    _prevTarget?: Node;
    _nextTarget?: Node;
    _version: number;
    _rollbackNode?: Node;
};
declare function batch<T>(callback: () => T): T;
declare function untracked<T>(callback: () => T): T;
declare class Signal<T = any> {
    /** @internal */
    _value: unknown;
    /**
     * @internal
     * Version numbers should always be >= 0, because the special value -1 is used
     * by Nodes to signify potentially unused but recyclable nodes.
     */
    _version: number;
    /** @internal */
    _node?: Node;
    /** @internal */
    _targets?: Node;
    constructor(value?: T);
    /** @internal */
    _refresh(): boolean;
    /** @internal */
    _subscribe(node: Node): void;
    /** @internal */
    _unsubscribe(node: Node): void;
    subscribe(fn: (value: T) => void): () => void;
    valueOf(): T;
    toString(): string;
    toJSON(): T;
    peek(): T;
    brand: typeof identifier;
    get value(): T;
    set value(value: T);
}
/** @internal */
declare function Signal(this: Signal, value?: unknown): void;
declare function signal<T>(value: T): Signal<T>;
declare class Computed<T = any> extends Signal<T> {
    _compute: () => T;
    _sources?: Node;
    _globalVersion: number;
    _flags: number;
    constructor(compute: () => T);
    _notify(): void;
    get value(): T;
}
declare function Computed(this: Computed, compute: () => unknown): void;
declare namespace Computed {
    var prototype: Computed<any>;
}
interface ReadonlySignal<T = any> extends Signal<T> {
    readonly value: T;
}
declare function computed<T>(compute: () => T): ReadonlySignal<T>;
type EffectCleanup = () => unknown;
declare class Effect {
    _compute?: () => unknown | EffectCleanup;
    _cleanup?: () => unknown;
    _sources?: Node;
    _nextBatchedEffect?: Effect;
    _flags: number;
    constructor(compute: () => unknown | EffectCleanup);
    _callback(): void;
    _start(): () => void;
    _notify(): void;
    _dispose(): void;
}
declare function Effect(this: Effect, compute: () => unknown | EffectCleanup): void;
declare function effect(compute: () => unknown | EffectCleanup): () => void;

