/// <reference types="../../codemirror-style.d.ts"/>


const IDENT_CHAR_RE = /[-_a-zA-Z0-9]/;
const STARTING_IDENT_CHAR_RE = /[_a-zA-Z]/;

CodeMirror.defineMode("inox", (config, parserConfig) => {
  let indentUnit = config.indentUnit;
  let statementIndent = parserConfig.statementIndent;
  let trackScope = parserConfig.trackScope !== false;
  let isTS = parserConfig.typescript;

  // Used as scratch variables to communicate multiple values without
  // consing up tons of objects.
  let type, content;

  /** @type {CodeMirror.Mode<{}>} */
  let mode = {
    startState: () => {
        return {}
    },

    token: (stream, state) => {
        return tokenize(stream, state)
    },

    lineComment: "# ",
    fold: "brace",
    closeBrackets: "()[]{}''\"\"``",
  };
  return mode
});

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
    "if": keyword("if"),
    "else": A,

    "return": B,
    "break": B,
    "continue": B,

    "var": keyword("var"),
    "const": keyword("var"),
    "fn": keyword("function"),
    "for": keyword("for"),
    "switch": keyword("switch"),
    "self": keyword("this"),
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
 * @param {CodeMirror.StringStream} stream 
 * @param {any} state 
 * @returns {(style_name | null)}
 */
const tokenize = (stream, state) => {
    let ch = stream.next();
    if(ch == null){
        return null
    }

    if (/\d/.test(ch)) {
      stream.match(/^[\d_]*(?:n|(?:\.[\d_]*)?(?:[eE][+\-]?[\d_]+)?)?/);
      return 'number'
    }

    if(STARTING_IDENT_CHAR_RE.test(ch)){
      stream.eatWhile(IDENT_CHAR_RE);
      let word = stream.current()
      if(word in keywords){
        //@ts-ignore
        return keywords[word].style
      }
      return 'variable'
    }

    switch(ch){
    case '{': case '}': case '(': case ')': case '[': case ']':
      return 'bracket'
    case ';':
      return 'punctuation'
    }

    return 'variable'
}

