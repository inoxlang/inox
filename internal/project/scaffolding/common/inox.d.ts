/**
 * @param {{
 *      signals?: Record<string, Signal>
 * }} arg
 */
declare function initComponent(arg: {
    signals?: Record<string, Signal> | undefined;
}): void;
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
 *  @property {() => void} rerender
 */
/**
 * @param {string} rawInterpolation
 * @param {number} interpolationPosition
 * @param {Text} node
 * */
declare function getInterpolation(rawInterpolation: string, interpolationPosition: number, node: Text): Interpolation;
/**
 * @param {Text} node
 * @param {Interpolation[]} interpolations
 */
declare function makeRenderTextNode(node: Text, interpolations: Interpolation[]): () => void;
/**e
 * @param {Interpolation} interpolation
 * @param {Record<string, string>} state
 */
declare function renderInterpolatio(interpolation: Interpolation, state: Record<string, string>): void;
/**
 * @param {Node} elem
 * @param {(n: Node) => any} visit
 */
declare function walkNode(elem: Node, visit: (n: Node) => any): void;
declare const INTERPOLATION_PATTERN: RegExp;
/** @type {WeakMap<Signal, Dependent[]>} */
declare const signalsToDependents: WeakMap<Signal, Dependent[]>;
/** @type {WeakMap<Text, Dependent>} */
declare const textsWithInterpolations: WeakMap<Text, Dependent>;
type Interpolation = {
    node: Text;
    name: string;
    startIndex: number;
    endIndex: number;
    default?: string | undefined;
    type?: string | undefined;
};
type Dependent = TextNodeDependent;
type TextNodeDependent = {
    type: "text";
    node: Text;
    interpolations: Interpolation[];
    rerender: () => void;
};
