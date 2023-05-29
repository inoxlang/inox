/// <reference types="../../codemirror-style.d.ts"/>

const IDENT_CHAR_RE = /[-_a-zA-Z0-9]/;
const STARTING_IDENT_CHAR_RE = /[_a-zA-Z]/;
const LOWERCASE_A_CODE = 'a'.codePointAt(0)
const LOWERCASE_Z_CODE = 'z'.codePointAt(0)

/**
 * @typedef State
 * @property {PreviousElement} prevElem
 * @property {string[]} stack
 * @property {boolean} inMultilineString
 * @property {boolean} isFirstLineCharInMultineString
 * @property {(style: style_name, prev: PreviousElement) => style_name} returnSetPrev
 */

/**
 * @typedef {'line-start' | 'expr' | 'space'} PreviousElement
 */

CodeMirror.defineMode("inox", (config, parserConfig) => {
  let indentUnit = config.indentUnit;
  let statementIndent = parserConfig.statementIndent;
  let trackScope = parserConfig.trackScope !== false;
  let isTS = parserConfig.typescript;

  // Used as scratch variables to communicate multiple values without
  // consing up tons of objects.
  let type, content;

  /** @type {CodeMirror.Mode<State>} */
  let mode = {
    /** @returns {State} */
    startState: () => ({
      inMultilineString: false,
      prevElem: "line-start",
      isFirstLineCharInMultineString: false,
      stack: [],

      returnSetPrev(style, prev) {
        this.prevElem = prev;
        return style;
      },
    }),

    token: (stream, state) => {
      return tokenize(stream, state);
    },

    lineComment: "# ",
    fold: "brace",
    closeBrackets: "()[]{}''\"\"``",
  };
  return mode;
});

/**
 * @param {CodeMirror.StringStream} stream
 * @param {State} state
 * @returns {(style_name | null)}
 */
const tokenize = (stream, state) => {
  let ch = stream.next();
  if (ch == null) {
    return null;
  }

  if(state.inMultilineString){
    if(ch == '`'){ //line starts with `
      state.inMultilineString = false
      return 'string'
    }

    state.isFirstLineCharInMultineString = false

    let escaped = false, next;
      while ((next = stream.next()) != null) {
        if (next == '`' && !escaped) break;
        escaped = !escaped && next == "\\";
      }
      if(next == '`'){ 
        state.inMultilineString = false
      }
    return 'string'
  }

  switch (ch) {
    case " ":
    case "\t":
      state.prevElem = "space";
      return null;
    case "\n":
      state.prevElem = "line-start";
    case '"':
      return tokenizeString('"', stream, state);
    case "`":
      return tokenizeString('`', stream, state);
    case "'":
      //TODO
      return tokenizeString("'", stream, state);
    case "%": {
      if (stream.match(/(?<!\w)[a-zA-Z_][a-zA-Z0-9_-]*\b/)) {
        return "type";
      }
      switch (stream.peek()) {
        case "{":
        case "[":
          return "punctuation";
      }
      break;
    }
    case "$":
      return tokenizeVariable(stream, state);
    case "_":
      return tokenizeIdentLike(stream, state);
    case "#":
      if (stream.match(/^[ \t!].*$/)) {
        return state.returnSetPrev("comment", "line-start");
      }
      return tokenizeUnambigousIdentifier(stream, state);
    case ";":
    case ":":
    case ",":
      return "punctuation";
    case ".":
    case "/":
      if (stream.match(/\.{0,2}\/[-a-zA-Z0-9_+@/.]*/)) { //path
        return "string";
      }

      if (ch == "/") {
        break;
      }

      //.

      if (isAlpha(stream.peek())) {
        return tokenizePropertyName(stream, state);
      }

      switch (stream.peek()) {
        case "_":
          return tokenizePropertyName(stream, state);
        case "?":
          stream.next()
          return tokenizePropertyName(stream, state);
        case "(":
          return "punctuation";
      }

      if (stream.match("..")) {
        return "punctuation";
      }

      break;
    case "+":
    case "*":
    case "\\":
    case "=":
    case ">":
    case "<":
    case "!":
    case "?":
      return "operator";
    case "-":
      if (!stream.match(/\d/, false)) {
        return null;
      }
      //number
    case "0":
    case "1":
    case "2":
    case "3":
    case "4":
    case "5":
    case "6":
    case "7":
    case "8":
    case "9":
      if (stream.match(/^-?[\d_]*(?:n|(?:\.[\d_]*)?(?:[eE][+\-]?[\d_]+)?)?/)) {
        return "number";
      }
      break;
    case "{":
    case "(":
    case "[":
      state.stack.push(ch);
      return "bracket";
    case "}":
    case ")":
    case "]":
      if (getOpeningDelim(ch) == state.stack[state.stack.length - 1]) {
        state.stack.pop();
      }
      state.prevElem = "expr";
      return "bracket";
  }

  if (STARTING_IDENT_CHAR_RE.test(ch)) {
    return tokenizeIdentLike(stream, state);
  }

  return "invalidchar";
};

/**
 * @param {string} quote
 * @param {CodeMirror.StringStream} stream
 * @param {State} state
 * @returns {style_name}
 */
const tokenizeString = (quote, stream, state) => {
  if(quote == '`'){
    state.inMultilineString = true;
  }
  
  let escaped = false, next;
  while ((next = stream.next()) != null) {
    if (next == quote && !escaped) break;
    escaped = !escaped && next == "\\";
  }
  if(next == quote){ 
    state.inMultilineString = false
  }
  return "string";
};

/**
 * @param {CodeMirror.StringStream} stream
 * @param {State} state
 * @returns {style_name}
 */
const tokenizeIdentLike = (stream, state) => {
  stream.eatWhile(IDENT_CHAR_RE);
  let word = stream.current();
  if (word in keywords) {
    //@ts-ignore
    return state.returnSetPrev(keywords[word].style);
  }
  switch (word) {
    case "true":
    case "false":
    case "nil":
      return state.returnSetPrev("atom", "expr");
  }
  return state.returnSetPrev("variable", "expr");
};

/**
 * @param {CodeMirror.StringStream} stream
 * @param {State} state
 * @returns {style_name}
 */
const tokenizeVariable = (stream, state) => {
  stream.eatWhile(IDENT_CHAR_RE);
  return state.returnSetPrev("variable", "expr");
};

/**
 * @param {CodeMirror.StringStream} stream
 * @param {State} state
 * @returns {style_name}
 */
const tokenizePropertyName = (stream, state) => {
  stream.eatWhile(IDENT_CHAR_RE);
  if (state.prevElem == "expr") {
    return state.returnSetPrev("property", "expr");
  }
  return state.returnSetPrev("atom", "expr");
};

/**
 * @param {CodeMirror.StringStream} stream
 * @param {State} state
 * @returns {style_name}
 */
const tokenizeUnambigousIdentifier = (stream, state) => {
  stream.eatWhile(IDENT_CHAR_RE);

  return state.returnSetPrev("atom", "expr");
};

const keywords = (() => {
  /** @param {string} type */
  function keyword(type) {
    return { type: type, style: "keyword" };
  }

  var A = keyword("keyword a"),
    B = keyword("keyword b"),
    operator = keyword("operator"),
    atom = { type: "atom", style: "atom" };

  return {
    "manifest": keyword("manifest"),
    "if": keyword("if"),
    "assert": keyword("assert"),
    "else": A,

    "return": B,
    "break": B,
    "continue": B,

    "var": keyword("var"),
    "const": keyword("var"),
    "fn": keyword("function"),
    "for": keyword("for"),
    "switch": keyword("switch"),
    "self": keyword("self"),
    "import": keyword("import"),

    //atoms
    "true": atom,
    "false": atom,
    "nil": atom,

    //operators
    "in": operator,
    "not-in": operator,
    "keyof": operator,
    "and": operator,
    "or": operator,
    //TODO: finish
  };
})();

/**
 * @param {string} closingDelim
 */
function getOpeningDelim(closingDelim) {
  switch (closingDelim) {
    case "}":
      return "{";
    case "]":
      return "[";
    case ")":
      return "(";
    default:
      throw new Error("unreachable");
  }
}

/**
 * @param {string} ch
 */
function isAlpha(ch) {
  let code = ch.toLowerCase().codePointAt(0)
  return code >= LOWERCASE_A_CODE && code <= LOWERCASE_Z_CODE
}
