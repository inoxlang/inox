//This file contains lexing and parsing logic that has been extracted from https://github.com/bigskysoftware/_hyperscript/blob/898345a1753ec365491dd6eedc3ab06873862109/src/_hyperscript.js#L1849 

function parseHyperScript(args = {}){
    let input = args.input
    const doNotIncludeNodeData = args.doNotIncludeNodeData || false

    if(input == undefined){
        input = globalThis.input
    }

    const tokens = Lexer.tokenize(input)
    const tokenList = Array.from(tokens.tokens).map(jsonifyToken) //Get tokens before they are used by the parser.

    try {
        const node = parser.parseHyperScript(tokens)

        let nodeData = node
        if(doNotIncludeNodeData){
            nodeData = {}
        }

        return {
            outputJSON: JSON.stringify({ 
                nodeData: nodeData, 
                tokens: tokenList
            })
        }
    } catch (err) {

        if (err instanceof ParsingError) {
            return {
                errorJSON: JSON.stringify({
                    message: err.message,
                    messageAtToken: err.messageAtToken,
                    token: jsonifyToken(err.token),
                    tokens: tokenList
                })
            }
        } else {
            return {
                criticalError: err.message
            }
        }
    }
}

const config = {
    attributes: "_, script, data-script",
    defaultTransition: "all 500ms ease-in",
    disableSelector: "[disable-scripting], [data-disable-scripting]",
    hideShowStrategies: {},
    //conversions,
}

/** @typedef {any} Context */
/** @typedef {any} Runtime */

const HALT = {};

class Lexer {
    static OP_TABLE = {
        "+": "PLUS",
        "-": "MINUS",
        "*": "MULTIPLY",
        "/": "DIVIDE",
        ".": "PERIOD",
        "..": "ELLIPSIS",
        "\\": "BACKSLASH",
        ":": "COLON",
        "%": "PERCENT",
        "|": "PIPE",
        "!": "EXCLAMATION",
        "?": "QUESTION",
        "#": "POUND",
        "&": "AMPERSAND",
        $: "DOLLAR",
        ";": "SEMI",
        ",": "COMMA",
        "(": "L_PAREN",
        ")": "R_PAREN",
        "<": "L_ANG",
        ">": "R_ANG",
        "<=": "LTE_ANG",
        ">=": "GTE_ANG",
        "==": "EQ",
        "===": "EQQ",
        "!=": "NEQ",
        "!==": "NEQQ",
        "{": "L_BRACE",
        "}": "R_BRACE",
        "[": "L_BRACKET",
        "]": "R_BRACKET",
        "=": "EQUALS",
    };

    /**
     * isValidCSSClassChar returns `true` if the provided character is valid in a CSS class.
     * @param {string} c
     * @returns boolean
     */
    static isValidCSSClassChar(c) {
        return Lexer.isAlpha(c) || Lexer.isNumeric(c) || c === "-" || c === "_" || c === ":";
    }

    /**
     * isValidCSSIDChar returns `true` if the provided character is valid in a CSS ID
     * @param {string} c
     * @returns boolean
     */
    static isValidCSSIDChar(c) {
        return Lexer.isAlpha(c) || Lexer.isNumeric(c) || c === "-" || c === "_" || c === ":";
    }

    /**
     * isWhitespace returns `true` if the provided character is whitespace.
     * @param {string} c
     * @returns boolean
     */
    static isWhitespace(c) {
        return c === " " || c === "\t" || Lexer.isNewline(c);
    }

    /**
     * positionString returns a string representation of a Token's line and column details.
     * @param {Token} token
     * @returns string
     */
    static positionString(token) {
        return "[Line: " + token.line + ", Column: " + token.column + "]";
    }

    /**
     * isNewline returns `true` if the provided character is a carriage return or newline
     * @param {string} c
     * @returns boolean
     */
    static isNewline(c) {
        return c === "\r" || c === "\n";
    }

    /**
     * isNumeric returns `true` if the provided character is a number (0-9)
     * @param {string} c
     * @returns boolean
     */
    static isNumeric(c) {
        return c >= "0" && c <= "9";
    }

    /**
     * isAlpha returns `true` if the provided character is a letter in the alphabet
     * @param {string} c
     * @returns boolean
     */
    static isAlpha(c) {
        return (c >= "a" && c <= "z") || (c >= "A" && c <= "Z");
    }

    /**
     * @param {string} c
     * @param {boolean} [dollarIsOp]
     * @returns boolean
     */
    static isIdentifierChar(c, dollarIsOp) {
        return c === "_" || c === "$";
    }

    /**
     * @param {string} c
     * @returns boolean
     */
    static isReservedChar(c) {
        return c === "`" || c === "^";
    }

    /**
     * @param {Token[]} tokens
     * @returns {boolean}
     */
    static isValidSingleQuoteStringStart(tokens) {
        if (tokens.length > 0) {
            var previousToken = tokens[tokens.length - 1];
            if (
                previousToken.type === "IDENTIFIER" ||
                previousToken.type === "CLASS_REF" ||
                previousToken.type === "ID_REF"
            ) {
                return false;
            }
            if (previousToken.op && (previousToken.value === ">" || previousToken.value === ")")) {
                return false;
            }
        }
        return true;
    }

    /**
     * @param {string} string
     * @param {boolean} [template]
     * @returns {Tokens}
     */
    static tokenize(string, template) {
        var tokens = /** @type {Token[]}*/[];
        var source = string;
        var position = 0;
        var column = 0;
        var line = 1;
        var lastToken = "<START>";
        var templateBraceCount = 0;

        function inTemplate() {
            return template && templateBraceCount === 0;
        }

        while (position < source.length) {
            if ((currentChar() === "-" && nextChar() === "-" && (Lexer.isWhitespace(nextCharAt(2)) || nextCharAt(2) === "" || nextCharAt(2) === "-"))
                || (currentChar() === "/" && nextChar() === "/" && (Lexer.isWhitespace(nextCharAt(2)) || nextCharAt(2) === "" || nextCharAt(2) === "/"))) {
                consumeComment();
            } else if (currentChar() === "/" && nextChar() === "*" && (Lexer.isWhitespace(nextCharAt(2)) || nextCharAt(2) === "" || nextCharAt(2) === "*")) {
                consumeCommentMultiline();
            } else {
                if (Lexer.isWhitespace(currentChar())) {
                    tokens.push(consumeWhitespace());
                } else if (
                    !possiblePrecedingSymbol() &&
                    currentChar() === "." &&
                    (Lexer.isAlpha(nextChar()) || nextChar() === "{" || nextChar() === "-")
                ) {
                    tokens.push(consumeClassReference());
                } else if (
                    !possiblePrecedingSymbol() &&
                    currentChar() === "#" &&
                    (Lexer.isAlpha(nextChar()) || nextChar() === "{")
                ) {
                    tokens.push(consumeIdReference());
                } else if (currentChar() === "[" && nextChar() === "@") {
                    tokens.push(consumeAttributeReference());
                } else if (currentChar() === "@") {
                    tokens.push(consumeShortAttributeReference());
                } else if (currentChar() === "*" && Lexer.isAlpha(nextChar())) {
                    tokens.push(consumeStyleReference());
                } else if (Lexer.isAlpha(currentChar()) || (!inTemplate() && Lexer.isIdentifierChar(currentChar()))) {
                    tokens.push(consumeIdentifier());
                } else if (Lexer.isNumeric(currentChar())) {
                    tokens.push(consumeNumber());
                } else if (!inTemplate() && (currentChar() === '"' || currentChar() === "`")) {
                    tokens.push(consumeString());
                } else if (!inTemplate() && currentChar() === "'") {
                    if (Lexer.isValidSingleQuoteStringStart(tokens)) {
                        tokens.push(consumeString());
                    } else {
                        tokens.push(consumeOp());
                    }
                } else if (Lexer.OP_TABLE[currentChar()]) {
                    if (lastToken === "$" && currentChar() === "{") {
                        templateBraceCount++;
                    }
                    if (currentChar() === "}") {
                        templateBraceCount--;
                    }
                    tokens.push(consumeOp());
                } else if (inTemplate() || Lexer.isReservedChar(currentChar())) {
                    tokens.push(makeToken("RESERVED", consumeChar()));
                } else {
                    if (position < source.length) {
                        throw Error("unknown token: " + currentChar() + " ");
                    }
                }
            }
        }

        return new Tokens(tokens, [], source);

        /**
         * @param {string} [type]
         * @param {string} [value]
         * @returns {Token}
         */
        function makeOpToken(type, value) {
            var token = makeToken(type, value);
            token.op = true;
            return token;
        }

        /**
         * @param {string} [type]
         * @param {string} [value]
         * @returns {Token}
         */
        function makeToken(type, value) {
            return {
                type: type,
                value: value || "",
                start: position,
                end: position + 1,
                column: column,
                line: line,
            };
        }

        function consumeComment() {
            while (currentChar() && !Lexer.isNewline(currentChar())) {
                consumeChar();
            }
            consumeChar(); // Consume newline
        }

        function consumeCommentMultiline() {
            while (currentChar() && !(currentChar() === '*' && nextChar() === '/')) {
                consumeChar();
            }
            consumeChar(); // Consume "*/"
            consumeChar();
        }

        /**
         * @returns Token
         */
        function consumeClassReference() {
            var classRef = makeToken("CLASS_REF");
            var value = consumeChar();
            if (currentChar() === "{") {
                classRef.template = true;
                value += consumeChar();
                while (currentChar() && currentChar() !== "}") {
                    value += consumeChar();
                }
                if (currentChar() !== "}") {
                    throw Error("Unterminated class reference");
                } else {
                    value += consumeChar(); // consume final curly
                }
            } else {
                while (Lexer.isValidCSSClassChar(currentChar())) {
                    value += consumeChar();
                }
            }
            classRef.value = value;
            classRef.end = position;
            return classRef;
        }

        /**
         * @returns Token
         */
        function consumeAttributeReference() {
            var attributeRef = makeToken("ATTRIBUTE_REF");
            var value = consumeChar();
            while (position < source.length && currentChar() !== "]") {
                value += consumeChar();
            }
            if (currentChar() === "]") {
                value += consumeChar();
            }
            attributeRef.value = value;
            attributeRef.end = position;
            return attributeRef;
        }

        function consumeShortAttributeReference() {
            var attributeRef = makeToken("ATTRIBUTE_REF");
            var value = consumeChar();
            while (Lexer.isValidCSSIDChar(currentChar())) {
                value += consumeChar();
            }
            if (currentChar() === '=') {
                value += consumeChar();
                if (currentChar() === '"' || currentChar() === "'") {
                    let stringValue = consumeString();
                    value += stringValue.value;
                } else if (Lexer.isAlpha(currentChar()) ||
                    Lexer.isNumeric(currentChar()) ||
                    Lexer.isIdentifierChar(currentChar())) {
                    let id = consumeIdentifier();
                    value += id.value;
                }
            }
            attributeRef.value = value;
            attributeRef.end = position;
            return attributeRef;
        }

        function consumeStyleReference() {
            var styleRef = makeToken("STYLE_REF");
            var value = consumeChar();
            while (Lexer.isAlpha(currentChar()) || currentChar() === "-") {
                value += consumeChar();
            }
            styleRef.value = value;
            styleRef.end = position;
            return styleRef;
        }

        /**
         * @returns Token
         */
        function consumeIdReference() {
            var idRef = makeToken("ID_REF");
            var value = consumeChar();
            if (currentChar() === "{") {
                idRef.template = true;
                value += consumeChar();
                while (currentChar() && currentChar() !== "}") {
                    value += consumeChar();
                }
                if (currentChar() !== "}") {
                    throw Error("Unterminated id reference");
                } else {
                    consumeChar(); // consume final quote
                }
            } else {
                while (Lexer.isValidCSSIDChar(currentChar())) {
                    value += consumeChar();
                }
            }
            idRef.value = value;
            idRef.end = position;
            return idRef;
        }

        /**
         * @returns Token
         */
        function consumeIdentifier() {
            var identifier = makeToken("IDENTIFIER");
            var value = consumeChar();
            while (Lexer.isAlpha(currentChar()) ||
                Lexer.isNumeric(currentChar()) ||
                Lexer.isIdentifierChar(currentChar())) {
                value += consumeChar();
            }
            if (currentChar() === "!" && value === "beep") {
                value += consumeChar();
            }
            identifier.value = value;
            identifier.end = position;
            return identifier;
        }

        /**
         * @returns Token
         */
        function consumeNumber() {
            var number = makeToken("NUMBER");
            var value = consumeChar();

            // given possible XXX.YYY(e|E)[-]ZZZ consume XXX
            while (Lexer.isNumeric(currentChar())) {
                value += consumeChar();
            }

            // consume .YYY
            if (currentChar() === "." && Lexer.isNumeric(nextChar())) {
                value += consumeChar();
            }
            while (Lexer.isNumeric(currentChar())) {
                value += consumeChar();
            }

            // consume (e|E)[-]
            if (currentChar() === "e" || currentChar() === "E") {
                // possible scientific notation, e.g. 1e6 or 1e-6
                if (Lexer.isNumeric(nextChar())) {
                    // e.g. 1e6
                    value += consumeChar();
                } else if (nextChar() === "-") {
                    // e.g. 1e-6
                    value += consumeChar();
                    // consume the - as well since otherwise we would stop on the next loop
                    value += consumeChar();
                }
            }

            // consume ZZZ
            while (Lexer.isNumeric(currentChar())) {
                value += consumeChar();
            }
            number.value = value;
            number.end = position;
            return number;
        }

        /**
         * @returns Token
         */
        function consumeOp() {
            var op = makeOpToken();
            var value = consumeChar(); // consume leading char
            while (currentChar() && Lexer.OP_TABLE[value + currentChar()]) {
                value += consumeChar();
            }
            op.type = Lexer.OP_TABLE[value];
            op.value = value;
            op.end = position;
            return op;
        }

        /**
         * @returns Token
         */
        function consumeString() {
            var string = makeToken("STRING");
            var startChar = consumeChar(); // consume leading quote
            var value = "";
            while (currentChar() && currentChar() !== startChar) {
                if (currentChar() === "\\") {
                    consumeChar(); // consume escape char and get the next one
                    let nextChar = consumeChar();
                    if (nextChar === "b") {
                        value += "\b";
                    } else if (nextChar === "f") {
                        value += "\f";
                    } else if (nextChar === "n") {
                        value += "\n";
                    } else if (nextChar === "r") {
                        value += "\r";
                    } else if (nextChar === "t") {
                        value += "\t";
                    } else if (nextChar === "v") {
                        value += "\v";
                    } else if (nextChar === "x") {
                        const hex = consumeHexEscape();
                        if (Number.isNaN(hex)) {
                            throw Error("Invalid hexadecimal escape at " + Lexer.positionString(string));
                        }
                        value += String.fromCharCode(hex);
                    } else {
                        value += nextChar;
                    }
                } else {
                    value += consumeChar();
                }
            }
            if (currentChar() !== startChar) {
                throw Error("Unterminated string at " + Lexer.positionString(string));
            } else {
                consumeChar(); // consume final quote
            }
            string.value = value;
            string.end = position;
            string.template = startChar === "`";
            return string;
        }

        /**
         * @returns number
         */
        function consumeHexEscape() {
            const BASE = 16;
            if (!currentChar()) {
                return NaN;
            }
            let result = BASE * Number.parseInt(consumeChar(), BASE);
            if (!currentChar()) {
                return NaN;
            }
            result += Number.parseInt(consumeChar(), BASE);

            return result;
        }

        /**
         * @returns string
         */
        function currentChar() {
            return source.charAt(position);
        }

        /**
         * @returns string
         */
        function nextChar() {
            return source.charAt(position + 1);
        }

        function nextCharAt(number = 1) {
            return source.charAt(position + number);
        }

        /**
         * @returns string
         */
        function consumeChar() {
            lastToken = currentChar();
            position++;
            column++;
            return lastToken;
        }

        /**
         * @returns boolean
         */
        function possiblePrecedingSymbol() {
            return (
                Lexer.isAlpha(lastToken) ||
                Lexer.isNumeric(lastToken) ||
                lastToken === ")" ||
                lastToken === "\"" ||
                lastToken === "'" ||
                lastToken === "`" ||
                lastToken === "}" ||
                lastToken === "]"
            );
        }

        /**
         * @returns Token
         */
        function consumeWhitespace() {
            var whitespace = makeToken("WHITESPACE");
            var value = "";
            while (currentChar() && Lexer.isWhitespace(currentChar())) {
                if (Lexer.isNewline(currentChar())) {
                    column = 0;
                    line++;
                }
                value += consumeChar();
            }
            whitespace.value = value;
            whitespace.end = position;
            return whitespace;
        }
    }

    /**
     * @param {string} string
     * @param {boolean} [template]
     * @returns {Tokens}
     */
    tokenize(string, template) {
        return Lexer.tokenize(string, template)
    }
}

/**
 * @typedef Token
 * @property {string} [type]
 * @property {string} value
 * @property {number} [start]
 * @property {number} [end]
 * @property {number} [column]
 * @property {number} [line]
 * @property {boolean} [op] `true` if this token represents an operator
 * @property {boolean} [template] `true` if this token is a template, for class refs, id refs, strings
 */
class Tokens {
    /**
     * 
     * @param {Token[]} tokens 
     * @param {Token[]} consumed 
     * @param {string} source 
     */
    constructor(tokens, consumed, source) {
        this.tokens = tokens
        this.consumed = consumed
        this.source = source

        this.consumeWhitespace(); // consume initial whitespace
    }

    get list() {
        return this.tokens
    }

    /** @type Token | null */
    _lastConsumed = null;

    consumeWhitespace() {
        while (this.token(0, true).type === "WHITESPACE") {
            this.consumed.push(this.tokens.shift());
        }
    }

    /**
     * @param {Tokens} tokens
     * @param {*} error
     * @returns {never}
     */
    raiseError(tokens, error) {
        Parser.raiseParseError(tokens, error);
    }

    /**
     * @param {string} value
     * @returns {Token}
     */
    requireOpToken(value) {
        var token = this.matchOpToken(value);
        if (token) {
            return token;
        } else {
            this.raiseError(this, "Expected '" + value + "' but found '" + this.currentToken().value + "'");
        }
    }

    /**
     * @param {string} op1
     * @param {string} [op2]
     * @param {string} [op3]
     * @returns {Token | void}
     */
    matchAnyOpToken(op1, op2, op3) {
        for (var i = 0; i < arguments.length; i++) {
            var opToken = arguments[i];
            var match = this.matchOpToken(opToken);
            if (match) {
                return match;
            }
        }
    }

    /**
     * @param {string} op1
     * @param {string} [op2]
     * @param {string} [op3]
     * @returns {Token | void}
     */
    matchAnyToken(op1, op2, op3) {
        for (var i = 0; i < arguments.length; i++) {
            var opToken = arguments[i];
            var match = this.matchToken(opToken);
            if (match) {
                return match;
            }
        }
    }

    /**
     * @param {string} value
     * @returns {Token | void}
     */
    matchOpToken(value) {
        if (this.currentToken() && this.currentToken().op && this.currentToken().value === value) {
            return this.consumeToken();
        }
    }

    /**
     * @param {string} type1
     * @param {string} [type2]
     * @param {string} [type3]
     * @param {string} [type4]
     * @returns {Token}
     */
    requireTokenType(type1, type2, type3, type4) {
        var token = this.matchTokenType(type1, type2, type3, type4);
        if (token) {
            return token;
        } else {
            this.raiseError(this, "Expected one of " + JSON.stringify([type1, type2, type3]));
        }
    }

    /**
     * @param {string} type1
     * @param {string} [type2]
     * @param {string} [type3]
     * @param {string} [type4]
     * @returns {Token | void}
     */
    matchTokenType(type1, type2, type3, type4) {
        if (
            this.currentToken() &&
            this.currentToken().type &&
            [type1, type2, type3, type4].indexOf(this.currentToken().type) >= 0
        ) {
            return this.consumeToken();
        }
    }

    /**
     * @param {string} value
     * @param {string} [type]
     * @returns {Token}
     */
    requireToken(value, type) {
        var token = this.matchToken(value, type);
        if (token) {
            return token;
        } else {
            this.raiseError(this, "Expected '" + value + "' but found '" + this.currentToken().value + "'");
        }
    }

    /**
     * @param {string} value 
     * @param {*} peek 
     * @param {*} type 
     * @returns 
     */
    peekToken(value, peek, type) {
        peek = peek || 0;
        type = type || "IDENTIFIER";
        if (this.tokens[peek] && this.tokens[peek].value === value && this.tokens[peek].type === type) {
            return this.tokens[peek];
        }
    }

    /**
     * @param {string} value
     * @param {string} [type]
     * @returns {Token | void}
     */
    matchToken(value, type) {
        if (this.follows.indexOf(value) !== -1) {
            return; // disallowed token here
        }
        type = type || "IDENTIFIER";
        if (this.currentToken() && this.currentToken().value === value && this.currentToken().type === type) {
            return this.consumeToken();
        }
    }

    /**
     * @returns {Token}
     */
    consumeToken() {
        var match = this.tokens.shift();
        this.consumed.push(match);
        this._lastConsumed = match;
        this.consumeWhitespace(); // consume any whitespace
        return match;
    }

    /**
     * @param {string | null} value
     * @param {string | null} [type]
     * @returns {Token[]}
     */
    consumeUntil(value, type) {
        /** @type Token[] */
        var tokenList = [];
        var currentToken = this.token(0, true);

        while (
            (type == null || currentToken.type !== type) &&
            (value == null || currentToken.value !== value) &&
            currentToken.type !== "EOF"
        ) {
            var match = this.tokens.shift();
            this.consumed.push(match);
            tokenList.push(currentToken);
            currentToken = this.token(0, true);
        }
        this.consumeWhitespace(); // consume any whitespace
        return tokenList;
    }

    /**
     * @returns {string}
     */
    lastWhitespace() {
        if (this.consumed[this.consumed.length - 1] && this.consumed[this.consumed.length - 1].type === "WHITESPACE") {
            return this.consumed[this.consumed.length - 1].value;
        } else {
            return "";
        }
    }

    consumeUntilWhitespace() {
        return this.consumeUntil(null, "WHITESPACE");
    }

    /**
     * @returns {boolean}
     */
    hasMore() {
        return this.tokens.length > 0;
    }

    /**
     * @param {number} n
     * @param {boolean} [dontIgnoreWhitespace]
     * @returns {Token}
     */
    token(n, dontIgnoreWhitespace) {
        var /**@type {Token}*/ token;
        var i = 0;
        do {
            if (!dontIgnoreWhitespace) {
                while (this.tokens[i] && this.tokens[i].type === "WHITESPACE") {
                    i++;
                }
            }
            token = this.tokens[i];
            n--;
            i++;
        } while (n > -1);
        if (token) {
            return token;
        } else {
            /** @type {Token} */
            const EOF = {
                type: "EOF",
                value: "<<<EOF>>>",
                //------------------
                line: 1,
                column: 0,
                start: 1,
                end: 1,  
            };

            if(this._lastConsumed){
                const last = this._lastConsumed
                EOF.line = last.line
                EOF.column = last.column + (last.end - last.start) + 1
                EOF.start = last.end
                EOF.end = last.end + 1
            }

            return EOF
        }
    }

    /**
     * @returns {Token}
     */
    currentToken() {
        return this.token(0);
    }

    /**
     * @returns {Token | null}
     */
    lastMatch() {
        return this._lastConsumed;
    }

    /**
     * @returns {string}
     */
    static sourceFor = function () {
        return this.programSource.substring(this.startToken.start, this.endToken.end);
    }

    /**
     * @returns {string}
     */
    static lineFor = function () {
        return this.programSource.split("\n")[this.startToken.line - 1];
    }

    follows = [];

    pushFollow(str) {
        this.follows.push(str);
    }

    popFollow() {
        this.follows.pop();
    }

    clearFollows() {
        var tmp = this.follows;
        this.follows = [];
        return tmp;
    }

    restoreFollows(f) {
        this.follows = f;
    }
}


class ParsingError extends Error {

    /** @type {string} */
    messageAtToken;

    /** @type {Token} */
    token;

    /** @type {Token[]} */
    tokens;

    constructor(completeMessage, messageAtToken, token, tokens) {
        super(completeMessage)
        this.messageAtToken = messageAtToken
        this.token = token
        this.tokens = tokens ?? []
    }
}

/**
 * @callback ParseRule
 * @param {Parser} parser
 * @param {unknown} runtime
 * @param {Tokens} tokens
 * @param {*} [root]
 * @returns {ASTNode | undefined}
 *
 * @typedef {Object} ASTNode
 * @member {boolean} isFeature
 * @member {string} type
 * @member {any[]} args
 * @member {(this: ASTNode, ctx:Context, root:any, ...args:any) => any} op
 * @member {(this: ASTNode, context?:Context) => any} evaluate
 * @member {ASTNode} parent
 * @member {Set<ASTNode>} children
 * @member {ASTNode} root
 * @member {String} keyword
 * @member {Token} endToken
 * @member {ASTNode} next
 * @member {(context:Context) => ASTNode} resolveNext
 * @member {EventSource} eventSource
 * @member {(this: ASTNode) => void} install
 * @member {(this: ASTNode, context:Context) => void} execute
 * @member {(this: ASTNode, target: object, source: object, args?: Object) => void} apply
 *
 *
 */

class Parser {
    constructor() {
        this.possessivesDisabled = false

        /* ============================================================================================ */
        /* Core hyperscript Grammar Elements                                                            */
        /* ============================================================================================ */
        this.addGrammarElement("feature", function (parser, runtime, tokens) {
            if (tokens.matchOpToken("(")) {
                var featureElement = parser.requireElement("feature", tokens);
                tokens.requireOpToken(")");
                return featureElement;
            }

            var featureDefinition = parser.FEATURES[tokens.currentToken().value || ""];
            if (featureDefinition) {
                return featureDefinition(parser, runtime, tokens);
            }
        });

        this.addGrammarElement("command", function (parser, runtime, tokens) {
            if (tokens.matchOpToken("(")) {
                const commandElement = parser.requireElement("command", tokens);
                tokens.requireOpToken(")");
                return commandElement;
            }

            var commandDefinition = parser.COMMANDS[tokens.currentToken().value || ""];
            let commandElement;
            if (commandDefinition) {
                commandElement = commandDefinition(parser, runtime, tokens);
            } else if (tokens.currentToken().type === "IDENTIFIER") {
                commandElement = parser.parseElement("pseudoCommand", tokens);
            }
            if (commandElement) {
                return parser.parseElement("indirectStatement", tokens, commandElement);
            }

            return commandElement;
        });

        this.addGrammarElement("commandList", function (parser, runtime, tokens) {
            if (tokens.hasMore()) {
                var cmd = parser.parseElement("command", tokens);
                if (cmd) {
                    tokens.matchToken("then");
                    const next = parser.parseElement("commandList", tokens);
                    if (next) cmd.next = next;
                    return cmd;
                }
            }
            return {
                type: "emptyCommandListCommand",
            }
        });

        this.addGrammarElement("leaf", function (parser, runtime, tokens) {
            var result = parser.parseAnyOf(parser.LEAF_EXPRESSIONS, tokens);
            // symbol is last so it doesn't consume any constants
            if (result == null) {
                return parser.parseElement("symbol", tokens);
            }

            return result;
        });

        this.addGrammarElement("indirectExpression", function (parser, runtime, tokens, root) {
            for (var i = 0; i < parser.INDIRECT_EXPRESSIONS.length; i++) {
                var indirect = parser.INDIRECT_EXPRESSIONS[i];
                root.endToken = tokens.lastMatch();
                var result = parser.parseElement(indirect, tokens, root);
                if (result) {
                    return result;
                }
            }
            return root;
        });

        this.addGrammarElement("indirectStatement", function (parser, runtime, tokens, root) {
            if (tokens.matchToken("unless")) {
                root.endToken = tokens.lastMatch();
                var conditional = parser.requireElement("expression", tokens);
                var unless = {
                    type: "unlessStatementModifier",
                    args: [conditional],
                };
                root.parent = unless;
                return unless;
            }
            return root;
        });

        this.addGrammarElement("primaryExpression", function (parser, runtime, tokens) {
            var leaf = parser.parseElement("leaf", tokens);
            if (leaf) {
                return parser.parseElement("indirectExpression", tokens, leaf);
            }
            parser.raiseParseError(tokens, "Unexpected value: " + tokens.currentToken().value);
        });
    }

    use(plugin) {
        plugin(this)
        return this
    }

    /** @type {Object<string,ParseRule>} */
    GRAMMAR = {};

    /** @type {Object<string,ParseRule>} */
    COMMANDS = {};

    /** @type {Object<string,ParseRule>} */
    FEATURES = {};

    /** @type {string[]} */
    LEAF_EXPRESSIONS = [];
    /** @type {string[]} */
    INDIRECT_EXPRESSIONS = [];

    /**
     * @param {*} parseElement
     * @param {*} start
     * @param {Tokens} tokens
     */
    initElt(parseElement, start, tokens) {
        parseElement.startToken = start;
        //parseElement.sourceFor = Tokens.sourceFor;
        //parseElement.lineFor = Tokens.lineFor;
        //parseElement.programSource = tokens.source;
    }

    /**
     * @param {string} type
     * @param {Tokens} tokens
     * @param {ASTNode?} root
     * @returns {ASTNode}
     */
    parseElement(type, tokens, root = undefined) {
        var elementDefinition = this.GRAMMAR[type];
        if (elementDefinition) {
            var start = tokens.currentToken();
            var parseElement = elementDefinition(this, null/*runtime*/, tokens, root);
            if (parseElement) {
                this.initElt(parseElement, start, tokens);
                parseElement.endToken = parseElement.endToken || tokens.lastMatch();
                var root = parseElement.root;
                while (root != null) {
                    this.initElt(root, start, tokens);
                    root = root.root;
                }
            }
            return parseElement;
        }
    }

    /**
     * @param {string} type
     * @param {Tokens} tokens
     * @param {string} [message]
     * @param {*} [root]
     * @returns {ASTNode}
     */
    requireElement(type, tokens, message, root) {
        var result = this.parseElement(type, tokens, root);
        if (!result) Parser.raiseParseError(tokens, message || "Expected " + type);
        // @ts-ignore
        return result;
    }

    /**
     * @param {string[]} types
     * @param {Tokens} tokens
     * @returns {ASTNode}
     */
    parseAnyOf(types, tokens) {
        for (var i = 0; i < types.length; i++) {
            var type = types[i];
            var expression = this.parseElement(type, tokens);
            if (expression) {
                return expression;
            }
        }
    }

    /**
     * @param {string} name
     * @param {ParseRule} definition
     */
    addGrammarElement(name, definition) {
        this.GRAMMAR[name] = definition;
    }

    /**
     * @param {string} keyword
     * @param {ParseRule} definition
     */
    addCommand(keyword, definition) {
        var commandGrammarType = keyword + "Command";
        var commandDefinitionWrapper = function (parser, runtime, tokens) {
            const commandElement = definition(parser, runtime, tokens);
            if (commandElement) {
                commandElement.type = commandGrammarType;
                return commandElement;
            }
        };
        this.GRAMMAR[commandGrammarType] = commandDefinitionWrapper;
        this.COMMANDS[keyword] = commandDefinitionWrapper;
    }

    /**
     * @param {string} keyword
     * @param {ParseRule} definition
     */
    addFeature(keyword, definition) {
        var featureGrammarType = keyword + "Feature";

        /** @type {ParseRule} */
        var featureDefinitionWrapper = function (parser, runtime, tokens) {
            var featureElement = definition(parser, runtime, tokens);
            if (featureElement) {
                featureElement.isFeature = true;
                featureElement.keyword = keyword;
                featureElement.type = featureGrammarType;
                return featureElement;
            }
        };
        this.GRAMMAR[featureGrammarType] = featureDefinitionWrapper;
        this.FEATURES[keyword] = featureDefinitionWrapper;
    }

    /**
     * @param {string} name
     * @param {ParseRule} definition
     */
    addLeafExpression(name, definition) {
        this.LEAF_EXPRESSIONS.push(name);
        this.addGrammarElement(name, definition);
    }

    /**
     * @param {string} name
     * @param {ParseRule} definition
     */
    addIndirectExpression(name, definition) {
        this.INDIRECT_EXPRESSIONS.push(name);
        this.addGrammarElement(name, definition);
    }

    /**
     *
     * @param {Tokens} tokens
     * @returns string
     */
    static createParserContext(tokens) {
        var currentToken = tokens.currentToken();
        var source = tokens.source;
        var lines = source.split("\n");
        var line = currentToken && currentToken.line ? currentToken.line - 1 : lines.length - 1;
        var contextLine = lines[line];
        var offset = /** @type {number} */ (
            currentToken && currentToken.line ? currentToken.column : contextLine.length - 1);
        return contextLine + "\n" + " ".repeat(offset) + "^^\n\n";
    }

    /**
     * @param {Tokens} tokens
     * @param {string} [message]
     * @returns {never}
     */
    static raiseParseError(tokens, message) {
        const token = tokens.currentToken()

        const messageAtToken = message || "Unexpected Token : " + token.value
        const completeMessage = message ?? (messageAtToken + "\n\n" + Parser.createParserContext(tokens))
        var error = new ParsingError(completeMessage, messageAtToken, token, tokens.tokens);
        throw error;
    }

    /**
     * @param {Tokens} tokens
     * @param {string} [message]
     */
    raiseParseError(tokens, message) {
        Parser.raiseParseError(tokens, message)
    }

    /**
     * @param {Tokens} tokens
     * @returns {ASTNode}
     */
    parseHyperScript(tokens) {
        var result = this.parseElement("hyperscript", tokens);
        if (tokens.hasMore()) this.raiseParseError(tokens);
        if (result) return result;
    }

    /**
     * @param {ASTNode | undefined} elt
     * @param {ASTNode} parent
     */
    setParent(elt, parent) {
        if (typeof elt === 'object') {
            elt.parent = parent;
            if (typeof parent === 'object') {
                parent.children = (parent.children || new Set());
                parent.children.add(elt)
            }
            this.setParent(elt.next, parent);
        }
    }

    /**
     * @param {Token} token
     * @returns {ParseRule}
     */
    commandStart(token) {
        return this.COMMANDS[token.value || ""];
    }

    /**
     * @param {Token} token
     * @returns {ParseRule}
     */
    featureStart(token) {
        return this.FEATURES[token.value || ""];
    }

    /**
     * @param {Token} token
     * @returns {boolean}
     */
    commandBoundary(token) {
        if (
            token.value == "end" ||
            token.value == "then" ||
            token.value == "else" ||
            token.value == "otherwise" ||
            token.value == ")" ||
            this.commandStart(token) ||
            this.featureStart(token) ||
            token.type == "EOF"
        ) {
            return true;
        }
        return false;
    }

    /**
     * @param {Tokens} tokens
     * @returns {(string | ASTNode)[]}
     */
    parseStringTemplate(tokens) {
        /** @type {(string | ASTNode)[]} */
        var returnArr = [""];
        do {
            returnArr.push(tokens.lastWhitespace());
            if (tokens.currentToken().value === "$") {
                tokens.consumeToken();
                var startingBrace = tokens.matchOpToken("{");
                returnArr.push(this.requireElement("expression", tokens));
                if (startingBrace) {
                    tokens.requireOpToken("}");
                }
                returnArr.push("");
            } else if (tokens.currentToken().value === "\\") {
                tokens.consumeToken(); // skip next
                tokens.consumeToken();
            } else {
                var token = tokens.consumeToken();
                returnArr[returnArr.length - 1] += token ? token.value : "";
            }
        } while (tokens.hasMore());
        returnArr.push(tokens.lastWhitespace());
        return returnArr;
    }

    /**
     * @param {ASTNode} commandList
     */
    ensureTerminated(commandList) {
        var implicitReturn = {
            type: "implicitReturn",
        };

        var end = commandList;
        while (end.next) {
            end = end.next;
        }
        end.next = implicitReturn;
    }
}




/**
 * logError writes an error message to the Javascript console.  It can take any
 * value, but msg should commonly be a simple string.
 * @param {*} msg
 */
function logError(msg) {
    if (console.error) {
        console.error(msg);
    } else if (console.log) {
        console.log("ERROR: ", msg);
    }
}

// TODO: JSDoc description of what's happening here
function varargConstructor(Cls, args) {
    return new (Cls.bind.apply(Cls, [Cls].concat(args)))();
}

// Grammar

/**
 * @param {Parser} parser
 */
function hyperscriptCoreGrammar(parser) {
    parser.addLeafExpression("parenthesized", function (parser, _runtime, tokens) {
        if (tokens.matchOpToken("(")) {
            var follows = tokens.clearFollows();
            try {
                var expr = parser.requireElement("expression", tokens);
            } finally {
                tokens.restoreFollows(follows);
            }
            tokens.requireOpToken(")");
            return expr;
        }
    });

    parser.addLeafExpression("string", function (parser, runtime, tokens) {
        var stringToken = tokens.matchTokenType("STRING");
        if (!stringToken) return;
        var rawValue = /** @type {string} */ (stringToken.value);
        /** @type {any[]} */
        var args;
        if (stringToken.template) {
            var innerTokens = Lexer.tokenize(rawValue, true);
            args = parser.parseStringTemplate(innerTokens);
        } else {
            args = [];
        }
        return {
            type: "string",
            token: stringToken,
            args: args,
            evaluate: function (context) {
                throwOnlyParsingIsSupported()
            },
        };
    });

    parser.addGrammarElement("nakedString", function (parser, runtime, tokens) {
        if (tokens.hasMore()) {
            var tokenArr = tokens.consumeUntilWhitespace();
            tokens.matchTokenType("WHITESPACE");
            return {
                type: "nakedString",
                tokens: tokenArr,
                evaluate: function (context) {
                    throwOnlyParsingIsSupported()
                },
            };
        }
    });

    parser.addLeafExpression("number", function (parser, runtime, tokens) {
        var number = tokens.matchTokenType("NUMBER");
        if (!number) return;
        var numberToken = number;
        var value = parseFloat(/** @type {string} */(number.value));
        return {
            type: "number",
            value: value,
            numberToken: numberToken,
            evaluate: function () {
                return value;
            },
        };
    });

    parser.addLeafExpression("idRef", function (parser, runtime, tokens) {
        var elementId = tokens.matchTokenType("ID_REF");
        if (!elementId) return;
        if (!elementId.value) return;
        // TODO - unify these two expression types
        if (elementId.template) {
            var templateValue = elementId.value.substring(2);
            var innerTokens = Lexer.tokenize(templateValue);
            var innerExpression = parser.requireElement("expression", innerTokens);
            return {
                type: "idRefTemplate",
                args: [innerExpression],
                evaluate: function (context) {
                    throwOnlyParsingIsSupported()
                },
            };
        } else {
            const value = elementId.value.substring(1);
            return {
                type: "idRef",
                css: elementId.value,
                value: value,
                evaluate: function (context) {
                    throwOnlyParsingIsSupported()
                },
            };
        }
    });

    parser.addLeafExpression("classRef", function (parser, runtime, tokens) {
        var classRef = tokens.matchTokenType("CLASS_REF");

        if (!classRef) return;
        if (!classRef.value) return;

        // TODO - unify these two expression types
        if (classRef.template) {
            var templateValue = classRef.value.substring(2);
            var innerTokens = Lexer.tokenize(templateValue);
            var innerExpression = parser.requireElement("expression", innerTokens);
            return {
                type: "classRefTemplate",
                args: [innerExpression],
                evaluate: function (context) {
                    throwOnlyParsingIsSupported()
                },
            };
        } else {
            const css = classRef.value;
            return {
                type: "classRef",
                css: css,
                evaluate: function (context) {
                    throwOnlyParsingIsSupported()
                },
            };
        }
    });

    parser.addLeafExpression("queryRef", function (parser, runtime, tokens) {
        var queryStart = tokens.matchOpToken("<");
        if (!queryStart) return;
        var queryTokens = tokens.consumeUntil("/");
        tokens.requireOpToken("/");
        tokens.requireOpToken(">");
        var queryValue = queryTokens
            .map(function (t) {
                if (t.type === "STRING") {
                    return '"' + t.value + '"';
                } else {
                    return t.value;
                }
            })
            .join("");

        var template, innerTokens, args;
        if (queryValue.indexOf("$") >= 0) {
            template = true;
            innerTokens = Lexer.tokenize(queryValue, true);
            args = parser.parseStringTemplate(innerTokens);
        }

        return {
            type: "queryRef",
            css: queryValue,
            args: args,
            evaluate: function (context) {
                throwOnlyParsingIsSupported()
            },
        };
    });

    parser.addLeafExpression("attributeRef", function (parser, runtime, tokens) {
        var attributeRef = tokens.matchTokenType("ATTRIBUTE_REF");
        if (!attributeRef) return;
        if (!attributeRef.value) return;
        var outerVal = attributeRef.value;
        if (outerVal.indexOf("[") === 0) {
            var innerValue = outerVal.substring(2, outerVal.length - 1);
        } else {
            var innerValue = outerVal.substring(1);
        }
        var css = "[" + innerValue + "]";
        var split = innerValue.split("=");
        var name = split[0];
        var value = split[1];
        if (value) {
            // strip quotes
            if (value.indexOf('"') === 0) {
                value = value.substring(1, value.length - 1);
            }
        }
        return {
            type: "attributeRef",
            name: name,
            css: css,
            value: value,
            evaluate: function (context) {
                throwOnlyParsingIsSupported()
            },
        };
    });

    parser.addLeafExpression("styleRef", function (parser, runtime, tokens) {
        var styleRef = tokens.matchTokenType("STYLE_REF");
        if (!styleRef) return;
        if (!styleRef.value) return;
        var styleProp = styleRef.value.substr(1);
        if (styleProp.startsWith("computed-")) {
            styleProp = styleProp.substr("computed-".length);
            return {
                type: "computedStyleRef",
                name: styleProp,
                evaluate: function (context) {
                    throwOnlyParsingIsSupported()
                },
            };
        } else {
            return {
                type: "styleRef",
                name: styleProp,
                evaluate: function (context) {
                    throwOnlyParsingIsSupported()
                },
            };
        }
    });

    parser.addGrammarElement("objectKey", function (parser, runtime, tokens) {
        var token;
        if ((token = tokens.matchTokenType("STRING"))) {
            return {
                type: "objectKey",
                key: token.value,
                evaluate: function () {
                    return token.value;
                },
            };
        } else if (tokens.matchOpToken("[")) {
            var expr = parser.parseElement("expression", tokens);
            tokens.requireOpToken("]");
            return {
                type: "objectKey",
                expr: expr,
                args: [expr],
                evaluate: function (context) {
                    throwOnlyParsingIsSupported()
                },
            };
        } else {
            var key = "";
            do {
                token = tokens.matchTokenType("IDENTIFIER") || tokens.matchOpToken("-");
                if (token) key += token.value;
            } while (token);
            return {
                type: "objectKey",
                key: key,
                evaluate: function () {
                    throwOnlyParsingIsSupported()
                },
            };
        }
    });

    parser.addLeafExpression("objectLiteral", function (parser, runtime, tokens) {
        if (!tokens.matchOpToken("{")) return;
        var keyExpressions = [];
        var valueExpressions = [];
        if (!tokens.matchOpToken("}")) {
            do {
                var name = parser.requireElement("objectKey", tokens);
                tokens.requireOpToken(":");
                var value = parser.requireElement("expression", tokens);
                valueExpressions.push(value);
                keyExpressions.push(name);
            } while (tokens.matchOpToken(","));
            tokens.requireOpToken("}");
        }
        return {
            type: "objectLiteral",
            args: [keyExpressions, valueExpressions],
            evaluate: function (context) {
                throwOnlyParsingIsSupported()
            },
        };
    });

    parser.addGrammarElement("nakedNamedArgumentList", function (parser, runtime, tokens) {
        var fields = [];
        var valueExpressions = [];
        if (tokens.currentToken().type === "IDENTIFIER") {
            do {
                var name = tokens.requireTokenType("IDENTIFIER");
                tokens.requireOpToken(":");
                var value = parser.requireElement("expression", tokens);
                valueExpressions.push(value);
                fields.push({ name: name, value: value });
            } while (tokens.matchOpToken(","));
        }
        return {
            type: "namedArgumentList",
            fields: fields,
            args: [valueExpressions],
            evaluate: function (context) {
                throwOnlyParsingIsSupported()
            },
        };
    });

    parser.addGrammarElement("namedArgumentList", function (parser, runtime, tokens) {
        if (!tokens.matchOpToken("(")) return;
        var elt = parser.requireElement("nakedNamedArgumentList", tokens);
        tokens.requireOpToken(")");
        return elt;
    });

    parser.addGrammarElement("symbol", function (parser, runtime, tokens) {
        /** @scope {SymbolScope} */
        var scope = "default";
        if (tokens.matchToken("global")) {
            scope = "global";
        } else if (tokens.matchToken("element") || tokens.matchToken("module")) {
            scope = "element";
            // optional possessive
            if (tokens.matchOpToken("'")) {
                tokens.requireToken("s");
            }
        } else if (tokens.matchToken("local")) {
            scope = "local";
        }

        // TODO better look ahead here
        let eltPrefix = tokens.matchOpToken(":");
        let identifier = tokens.matchTokenType("IDENTIFIER");
        if (identifier && identifier.value) {
            var name = identifier.value;
            if (eltPrefix) {
                name = ":" + name;
            }
            if (scope === "default") {
                if (name.indexOf("$") === 0) {
                    scope = "global";
                }
                if (name.indexOf(":") === 0) {
                    scope = "element";
                }
            }
            return {
                type: "symbol",
                token: identifier,
                scope: scope,
                name: name,
                evaluate: function (context) {
                    throwOnlyParsingIsSupported()
                },
            };
        }
    });

    parser.addGrammarElement("implicitMeTarget", function (parser, runtime, tokens) {
        return {
            type: "implicitMeTarget",
            evaluate: function (context) {
                return context.you || context.me;
            },
        };
    });

    parser.addLeafExpression("boolean", function (parser, runtime, tokens) {
        var booleanLiteral = tokens.matchToken("true") || tokens.matchToken("false");
        if (!booleanLiteral) return;
        const value = booleanLiteral.value === "true";
        return {
            type: "boolean",
            evaluate: function (context) {
                throwOnlyParsingIsSupported()
            },
        };
    });

    parser.addLeafExpression("null", function (parser, runtime, tokens) {
        if (tokens.matchToken("null")) {
            return {
                type: "null",
                evaluate: function (context) {
                    throwOnlyParsingIsSupported()
                },
            };
        }
    });

    parser.addLeafExpression("arrayLiteral", function (parser, runtime, tokens) {
        if (!tokens.matchOpToken("[")) return;
        var values = [];
        if (!tokens.matchOpToken("]")) {
            do {
                var expr = parser.requireElement("expression", tokens);
                values.push(expr);
            } while (tokens.matchOpToken(","));
            tokens.requireOpToken("]");
        }
        return {
            type: "arrayLiteral",
            values: values,
            args: [values],
            evaluate: function (context) {
                throwOnlyParsingIsSupported()
            },
        };
    });

    parser.addLeafExpression("blockLiteral", function (parser, runtime, tokens) {
        if (!tokens.matchOpToken("\\")) return;
        var args = [];
        var arg1 = tokens.matchTokenType("IDENTIFIER");
        if (arg1) {
            args.push(arg1);
            while (tokens.matchOpToken(",")) {
                args.push(tokens.requireTokenType("IDENTIFIER"));
            }
        }
        // TODO compound op token
        tokens.requireOpToken("-");
        tokens.requireOpToken(">");
        var expr = parser.requireElement("expression", tokens);
        return {
            type: "blockLiteral",
            args: args,
            expr: expr,
            evaluate: function (ctx) {
                throwOnlyParsingIsSupported()
            },
        };
    });

    parser.addIndirectExpression("propertyAccess", function (parser, runtime, tokens, root) {
        if (!tokens.matchOpToken(".")) return;
        var prop = tokens.requireTokenType("IDENTIFIER");
        var propertyAccess = {
            type: "propertyAccess",
            root: root,
            prop: prop,
            args: [root],
            evaluate: function (context) {
                throwOnlyParsingIsSupported()
            },
        };
        return parser.parseElement("indirectExpression", tokens, propertyAccess);
    });

    parser.addIndirectExpression("of", function (parser, runtime, tokens, root) {
        if (!tokens.matchToken("of")) return;
        var newRoot = parser.requireElement("unaryExpression", tokens);
        // find the urroot
        var childOfUrRoot = null;
        var urRoot = root;
        while (urRoot.root) {
            childOfUrRoot = urRoot;
            urRoot = urRoot.root;
        }
        if (urRoot.type !== "symbol" && urRoot.type !== "attributeRef" && urRoot.type !== "styleRef" && urRoot.type !== "computedStyleRef") {
            parser.raiseParseError(tokens, "Cannot take a property of a non-symbol: " + urRoot.type);
        }
        var attribute = urRoot.type === "attributeRef";
        var style = urRoot.type === "styleRef" || urRoot.type === "computedStyleRef";
        if (attribute || style) {
            var attributeElt = urRoot
        }
        var prop = urRoot.name;

        var propertyAccess = {
            type: "ofExpression",
            prop: urRoot.token,
            root: newRoot,
            attribute: attributeElt,
            expression: root,
            args: [newRoot],
            evaluate: function (context) {
                throwOnlyParsingIsSupported()
            },
        };

        if (urRoot.type === "attributeRef") {
            propertyAccess.attribute = urRoot;
        }
        if (childOfUrRoot) {
            childOfUrRoot.root = propertyAccess;
            childOfUrRoot.args = [propertyAccess];
        } else {
            root = propertyAccess;
        }

        return parser.parseElement("indirectExpression", tokens, root);
    });

    parser.addIndirectExpression("possessive", function (parser, runtime, tokens, root) {
        if (parser.possessivesDisabled) {
            return;
        }
        var apostrophe = tokens.matchOpToken("'");
        if (
            apostrophe ||
            (root.type === "symbol" &&
                (root.name === "my" || root.name === "its" || root.name === "your") &&
                (tokens.currentToken().type === "IDENTIFIER" || tokens.currentToken().type === "ATTRIBUTE_REF" || tokens.currentToken().type === "STYLE_REF"))
        ) {
            if (apostrophe) {
                tokens.requireToken("s");
            }

            var attribute, style, prop;
            attribute = parser.parseElement("attributeRef", tokens);
            if (attribute == null) {
                style = parser.parseElement("styleRef", tokens);
                if (style == null) {
                    prop = tokens.requireTokenType("IDENTIFIER");
                }
            }
            var propertyAccess = {
                type: "possessive",
                root: root,
                attribute: attribute || style,
                prop: prop,
                args: [root],
                evaluate: function (context) {
                    throwOnlyParsingIsSupported()
                },
            };
            return parser.parseElement("indirectExpression", tokens, propertyAccess);
        }
    });

    parser.addIndirectExpression("inExpression", function (parser, runtime, tokens, root) {
        if (!tokens.matchToken("in")) return;
        var target = parser.requireElement("unaryExpression", tokens);
        var propertyAccess = {
            type: "inExpression",
            root: root,
            args: [root, target],
            evaluate: function (context) {
                throwOnlyParsingIsSupported()
            },
        };
        return parser.parseElement("indirectExpression", tokens, propertyAccess);
    });

    parser.addIndirectExpression("asExpression", function (parser, runtime, tokens, root) {
        if (!tokens.matchToken("as")) return;
        tokens.matchToken("a") || tokens.matchToken("an");

        //var conversion = parser.requireElement("dotOrColonPath", tokens).evaluate(); // OK No promise
        parser.requireElement("dotOrColonPath", tokens)

        var propertyAccess = {
            type: "asExpression",
            root: root,
            args: [root],
            evaluate: function (context) {
                throwOnlyParsingIsSupported()
            },
        };
        return parser.parseElement("indirectExpression", tokens, propertyAccess);
    });

    parser.addIndirectExpression("functionCall", function (parser, runtime, tokens, root) {
        if (!tokens.matchOpToken("(")) return;
        var args = [];
        if (!tokens.matchOpToken(")")) {
            do {
                args.push(parser.requireElement("expression", tokens));
            } while (tokens.matchOpToken(","));
            tokens.requireOpToken(")");
        }

        if (root.root) {
            var functionCall = {
                type: "functionCall",
                root: root,
                argExressions: args,
                args: [root.root, args],
                evaluate: function (context) {
                    throwOnlyParsingIsSupported()
                },
            };
        } else {
            var functionCall = {
                type: "functionCall",
                root: root,
                argExressions: args,
                args: [root, args],
                evaluate: function (context) {
                    throwOnlyParsingIsSupported()
                },
            };
        }
        return parser.parseElement("indirectExpression", tokens, functionCall);
    });

    parser.addIndirectExpression("attributeRefAccess", function (parser, runtime, tokens, root) {
        var attribute = parser.parseElement("attributeRef", tokens);
        if (!attribute) return;
        var attributeAccess = {
            type: "attributeRefAccess",
            root: root,
            attribute: attribute,
            args: [root],
            evaluate: function (context) {
                throwOnlyParsingIsSupported()
            },
        };
        return attributeAccess;
    });

    parser.addIndirectExpression("arrayIndex", function (parser, runtime, tokens, root) {
        if (!tokens.matchOpToken("[")) return;
        var andBefore = false;
        var andAfter = false;
        var firstIndex = null;
        var secondIndex = null;

        if (tokens.matchOpToken("..")) {
            andBefore = true;
            firstIndex = parser.requireElement("expression", tokens);
        } else {
            firstIndex = parser.requireElement("expression", tokens);

            if (tokens.matchOpToken("..")) {
                andAfter = true;
                var current = tokens.currentToken();
                if (current.type !== "R_BRACKET") {
                    secondIndex = parser.parseElement("expression", tokens);
                }
            }
        }
        tokens.requireOpToken("]");

        var arrayIndex = {
            type: "arrayIndex",
            root: root,
            prop: firstIndex,
            firstIndex: firstIndex,
            secondIndex: secondIndex,
            args: [root, firstIndex, secondIndex],
            evaluate: function (context) {
                throwOnlyParsingIsSupported()
            },
        };

        return parser.parseElement("indirectExpression", tokens, arrayIndex);
    });

    // taken from https://drafts.csswg.org/css-values-4/#relative-length
    //        and https://drafts.csswg.org/css-values-4/#absolute-length
    //        (NB: we do not support `in` dues to conflicts w/ the hyperscript grammar)
    var STRING_POSTFIXES = [
        'em', 'ex', 'cap', 'ch', 'ic', 'rem', 'lh', 'rlh', 'vw', 'vh', 'vi', 'vb', 'vmin', 'vmax',
        'cm', 'mm', 'Q', 'pc', 'pt', 'px'
    ];
    parser.addGrammarElement("postfixExpression", function (parser, runtime, tokens) {
        var root = parser.parseElement("primaryExpression", tokens);

        let stringPosfix = tokens.matchAnyToken.apply(tokens, STRING_POSTFIXES) || tokens.matchOpToken("%");
        if (stringPosfix) {
            return {
                type: "stringPostfix",
                postfix: stringPosfix.value,
                args: [root],
                evaluate: function (context) {
                    throwOnlyParsingIsSupported()
                },
            };
        }

        var timeFactor = null;
        if (tokens.matchToken("s") || tokens.matchToken("seconds")) {
            timeFactor = 1000;
        } else if (tokens.matchToken("ms") || tokens.matchToken("milliseconds")) {
            timeFactor = 1;
        }
        if (timeFactor) {
            return {
                type: "timeExpression",
                time: root,
                factor: timeFactor,
                args: [root],
                evaluate: function (context) {
                    throwOnlyParsingIsSupported()
                },
            };
        }

        if (tokens.matchOpToken(":")) {
            var typeName = tokens.requireTokenType("IDENTIFIER");
            if (!typeName.value) return;
            var nullOk = !tokens.matchOpToken("!");
            return {
                type: "typeCheck",
                typeName: typeName,
                nullOk: nullOk,
                args: [root],
                evaluate: function (context) {
                    throwOnlyParsingIsSupported()
                },
            };
        } else {
            return root;
        }
    });

    parser.addGrammarElement("logicalNot", function (parser, runtime, tokens) {
        if (!tokens.matchToken("not")) return;
        var root = parser.requireElement("unaryExpression", tokens);
        return {
            type: "logicalNot",
            root: root,
            args: [root],
            evaluate: function (context) {
                throwOnlyParsingIsSupported()
            },
        };
    });

    parser.addGrammarElement("noExpression", function (parser, runtime, tokens) {
        if (!tokens.matchToken("no")) return;
        var root = parser.requireElement("unaryExpression", tokens);
        return {
            type: "noExpression",
            root: root,
            args: [root],
            evaluate: function (context) {
                throwOnlyParsingIsSupported()
            },
        };
    });

    parser.addLeafExpression("some", function (parser, runtime, tokens) {
        if (!tokens.matchToken("some")) return;
        var root = parser.requireElement("expression", tokens);
        return {
            type: "noExpression",
            root: root,
            args: [root],
            evaluate(context) {
                throwOnlyParsingIsSupported()
            },
        };
    });

    parser.addGrammarElement("negativeNumber", function (parser, runtime, tokens) {
        if (!tokens.matchOpToken("-")) return;
        var root = parser.requireElement("unaryExpression", tokens);
        return {
            type: "negativeNumber",
            root: root,
            args: [root],
            evaluate: function (context) {
                throwOnlyParsingIsSupported()
            },
        };
    });

    parser.addGrammarElement("unaryExpression", function (parser, runtime, tokens) {
        tokens.matchToken("the"); // optional "the"
        return parser.parseAnyOf(
            ["beepExpression", "logicalNot", "relativePositionalExpression", "positionalExpression", "noExpression", "negativeNumber", "postfixExpression"],
            tokens
        );
    });

    parser.addGrammarElement("beepExpression", function (parser, runtime, tokens) {
        if (!tokens.matchToken("beep!")) return;
        var expression = parser.parseElement("unaryExpression", tokens);
        if (expression) {
            expression['booped'] = true;
            var originalEvaluate = expression.evaluate;
            expression.evaluate = function (ctx) {
                throwOnlyParsingIsSupported()
            }
            return expression;
        }
    });

    parser.addGrammarElement("relativePositionalExpression", function (parser, runtime, tokens) {
        var op = tokens.matchAnyToken("next", "previous");
        if (!op) return;
        var forwardSearch = op.value === "next";

        var thingElt = parser.parseElement("expression", tokens);

        if (tokens.matchToken("from")) {
            tokens.pushFollow("in");
            try {
                var from = parser.requireElement("unaryExpression", tokens);
            } finally {
                tokens.popFollow();
            }
        } else {
            var from = parser.requireElement("implicitMeTarget", tokens);
        }

        var inSearch = false;
        var withinElt;
        if (tokens.matchToken("in")) {
            inSearch = true;
            var inElt = parser.requireElement("unaryExpression", tokens);
        } else if (tokens.matchToken("within")) {
            withinElt = parser.requireElement("unaryExpression", tokens);
        } else {
            //withinElt = document.body;
            withinElt = undefined;
        }

        var wrapping = false;
        if (tokens.matchToken("with")) {
            tokens.requireToken("wrapping")
            wrapping = true;
        }

        return {
            type: "relativePositionalExpression",
            from: from,
            forwardSearch: forwardSearch,
            inSearch: inSearch,
            wrapping: wrapping,
            inElt: inElt,
            withinElt: withinElt,
            operator: op.value,
            args: [thingElt, from, inElt, withinElt],
            evaluate: function (context) {
                throwOnlyParsingIsSupported()
            },
        }

    });

    parser.addGrammarElement("positionalExpression", function (parser, runtime, tokens) {
        var op = tokens.matchAnyToken("first", "last", "random");
        if (!op) return;
        tokens.matchAnyToken("in", "from", "of");
        var rhs = parser.requireElement("unaryExpression", tokens);
        const operator = op.value;
        return {
            type: "positionalExpression",
            rhs: rhs,
            operator: op.value,
            args: [rhs],
            evaluate: function (context) {
                throwOnlyParsingIsSupported()
            },
        };
    });

    parser.addGrammarElement("mathOperator", function (parser, runtime, tokens) {
        var expr = parser.parseElement("unaryExpression", tokens);
        var mathOp,
            initialMathOp = null;
        mathOp = tokens.matchAnyOpToken("+", "-", "*", "/") || tokens.matchToken('mod');
        while (mathOp) {
            initialMathOp = initialMathOp || mathOp;
            var operator = mathOp.value;
            if (initialMathOp.value !== operator) {
                parser.raiseParseError(tokens, "You must parenthesize math operations with different operators");
            }
            var rhs = parser.parseElement("unaryExpression", tokens);
            expr = {
                type: "mathOperator",
                lhs: expr,
                rhs: rhs,
                operator: operator,
                args: [expr, rhs],
                evaluate: function (context) {
                    throwOnlyParsingIsSupported()
                },
            };
            mathOp = tokens.matchAnyOpToken("+", "-", "*", "/") || tokens.matchToken('mod');
        }
        return expr;
    });

    parser.addGrammarElement("mathExpression", function (parser, runtime, tokens) {
        return parser.parseAnyOf(["mathOperator", "unaryExpression"], tokens);
    });

    parser.addGrammarElement("comparisonOperator", function (parser, runtime, tokens) {
        var expr = parser.parseElement("mathExpression", tokens);
        var comparisonToken = tokens.matchAnyOpToken("<", ">", "<=", ">=", "==", "===", "!=", "!==");
        var operator = comparisonToken ? comparisonToken.value : null;
        var hasRightValue = true; // By default, most comparisons require two values, but there are some exceptions.
        var typeCheck = false;

        if (operator == null) {
            if (tokens.matchToken("is") || tokens.matchToken("am")) {
                if (tokens.matchToken("not")) {
                    if (tokens.matchToken("in")) {
                        operator = "not in";
                    } else if (tokens.matchToken("a")) {
                        operator = "not a";
                        typeCheck = true;
                    } else if (tokens.matchToken("empty")) {
                        operator = "not empty";
                        hasRightValue = false;
                    } else {
                        if (tokens.matchToken("really")) {
                            operator = "!==";
                        } else {
                            operator = "!=";
                        }
                        // consume additional optional syntax
                        if (tokens.matchToken("equal")) {
                            tokens.matchToken("to");
                        }
                    }
                } else if (tokens.matchToken("in")) {
                    operator = "in";
                } else if (tokens.matchToken("a")) {
                    operator = "a";
                    typeCheck = true;
                } else if (tokens.matchToken("empty")) {
                    operator = "empty";
                    hasRightValue = false;
                } else if (tokens.matchToken("less")) {
                    tokens.requireToken("than");
                    if (tokens.matchToken("or")) {
                        tokens.requireToken("equal");
                        tokens.requireToken("to");
                        operator = "<=";
                    } else {
                        operator = "<";
                    }
                } else if (tokens.matchToken("greater")) {
                    tokens.requireToken("than");
                    if (tokens.matchToken("or")) {
                        tokens.requireToken("equal");
                        tokens.requireToken("to");
                        operator = ">=";
                    } else {
                        operator = ">";
                    }
                } else {
                    if (tokens.matchToken("really")) {
                        operator = "===";
                    } else {
                        operator = "==";
                    }
                    if (tokens.matchToken("equal")) {
                        tokens.matchToken("to");
                    }
                }
            } else if (tokens.matchToken("equals")) {
                operator = "==";
            } else if (tokens.matchToken("really")) {
                tokens.requireToken("equals")
                operator = "===";
            } else if (tokens.matchToken("exist") || tokens.matchToken("exists")) {
                operator = "exist";
                hasRightValue = false;
            } else if (tokens.matchToken("matches") || tokens.matchToken("match")) {
                operator = "match";
            } else if (tokens.matchToken("contains") || tokens.matchToken("contain")) {
                operator = "contain";
            } else if (tokens.matchToken("includes") || tokens.matchToken("include")) {
                operator = "include";
            } else if (tokens.matchToken("do") || tokens.matchToken("does")) {
                tokens.requireToken("not");
                if (tokens.matchToken("matches") || tokens.matchToken("match")) {
                    operator = "not match";
                } else if (tokens.matchToken("contains") || tokens.matchToken("contain")) {
                    operator = "not contain";
                } else if (tokens.matchToken("exist") || tokens.matchToken("exist")) {
                    operator = "not exist";
                    hasRightValue = false;
                } else if (tokens.matchToken("include")) {
                    operator = "not include";
                } else {
                    parser.raiseParseError(tokens, "Expected matches or contains");
                }
            }
        }

        if (operator) {
            // Do not allow chained comparisons, which is dumb
            var typeName, nullOk, rhs
            if (typeCheck) {
                typeName = tokens.requireTokenType("IDENTIFIER");
                nullOk = !tokens.matchOpToken("!");
            } else if (hasRightValue) {
                rhs = parser.requireElement("mathExpression", tokens);
                if (operator === "match" || operator === "not match") {
                    rhs = rhs.css ? rhs.css : rhs;
                }
            }
            var lhs = expr;
            expr = {
                type: "comparisonOperator",
                operator: operator,
                typeName: typeName,
                nullOk: nullOk,
                lhs: expr,
                rhs: rhs,
                args: [expr, rhs],
                evaluate: function (context) {
                    throwOnlyParsingIsSupported()
                },
            };
        }
        return expr;
    });

    parser.addGrammarElement("comparisonExpression", function (parser, runtime, tokens) {
        return parser.parseAnyOf(["comparisonOperator", "mathExpression"], tokens);
    });

    parser.addGrammarElement("logicalOperator", function (parser, runtime, tokens) {
        var expr = parser.parseElement("comparisonExpression", tokens);
        var logicalOp,
            initialLogicalOp = null;
        logicalOp = tokens.matchToken("and") || tokens.matchToken("or");
        while (logicalOp) {
            initialLogicalOp = initialLogicalOp || logicalOp;
            if (initialLogicalOp.value !== logicalOp.value) {
                parser.raiseParseError(tokens, "You must parenthesize logical operations with different operators");
            }
            var rhs = parser.requireElement("comparisonExpression", tokens);
            const operator = logicalOp.value;
            expr = {
                type: "logicalOperator",
                operator: operator,
                lhs: expr,
                rhs: rhs,
                args: [expr, rhs],
                evaluate: function (context) {
                    throwOnlyParsingIsSupported()
                },
            };
            logicalOp = tokens.matchToken("and") || tokens.matchToken("or");
        }
        return expr;
    });

    parser.addGrammarElement("logicalExpression", function (parser, runtime, tokens) {
        return parser.parseAnyOf(["logicalOperator", "mathExpression"], tokens);
    });

    parser.addGrammarElement("asyncExpression", function (parser, runtime, tokens) {
        if (tokens.matchToken("async")) {
            var value = parser.requireElement("logicalExpression", tokens);
            var expr = {
                type: "asyncExpression",
                value: value,
                evaluate: function (context) {
                    return {
                        asyncWrapper: true,
                        value: this.value.evaluate(context), //OK
                    };
                },
            };
            return expr;
        } else {
            return parser.parseElement("logicalExpression", tokens);
        }
    });

    parser.addGrammarElement("expression", function (parser, runtime, tokens) {
        tokens.matchToken("the"); // optional the
        return parser.parseElement("asyncExpression", tokens);
    });

    parser.addGrammarElement("assignableExpression", function (parser, runtime, tokens) {
        tokens.matchToken("the"); // optional the

        // TODO obviously we need to generalize this as a left hand side / targetable concept
        var expr = parser.parseElement("primaryExpression", tokens);
        if (expr && (
            expr.type === "symbol" ||
            expr.type === "ofExpression" ||
            expr.type === "propertyAccess" ||
            expr.type === "attributeRefAccess" ||
            expr.type === "attributeRef" ||
            expr.type === "styleRef" ||
            expr.type === "arrayIndex" ||
            expr.type === "possessive")
        ) {
            return expr;
        } else {
            parser.raiseParseError(
                tokens,
                "A target expression must be writable.  The expression type '" + (expr && expr.type) + "' is not."
            );
        }
        return expr;
    });

    parser.addGrammarElement("hyperscript", function (parser, runtime, tokens) {
        var features = [];

        if (tokens.hasMore()) {
            while (parser.featureStart(tokens.currentToken()) || tokens.currentToken().value === "(") {
                var feature = parser.requireElement("feature", tokens);
                features.push(feature);
                tokens.matchToken("end"); // optional end
            }
        }
        return {
            type: "hyperscript",
            features: features,
        };
    });

    var parseEventArgs = function (tokens) {
        var args = [];
        // handle argument list (look ahead 3)
        if (
            tokens.token(0).value === "(" &&
            (tokens.token(1).value === ")" || tokens.token(2).value === "," || tokens.token(2).value === ")")
        ) {
            tokens.matchOpToken("(");
            do {
                args.push(tokens.requireTokenType("IDENTIFIER"));
            } while (tokens.matchOpToken(","));
            tokens.requireOpToken(")");
        }
        return args;
    };

    parser.addFeature("on", function (parser, runtime, tokens) {
        if (!tokens.matchToken("on")) return;
        var every = false;
        if (tokens.matchToken("every")) {
            every = true;
        }
        var events = [];
        var displayName = null;
        do {
            var on = parser.requireElement("eventName", tokens, "Expected event name");

            var eventName = on.evaluate(); // OK No Promise

            if (displayName) {
                displayName = displayName + " or " + eventName;
            } else {
                displayName = "on " + eventName;
            }
            var args = parseEventArgs(tokens);

            var filter = null;
            if (tokens.matchOpToken("[")) {
                filter = parser.requireElement("expression", tokens);
                tokens.requireOpToken("]");
            }

            var startCount, endCount, unbounded;
            if (tokens.currentToken().type === "NUMBER") {
                var startCountToken = tokens.consumeToken();
                if (!startCountToken.value) return;
                startCount = parseInt(startCountToken.value);
                if (tokens.matchToken("to")) {
                    var endCountToken = tokens.consumeToken();
                    if (!endCountToken.value) return;
                    endCount = parseInt(endCountToken.value);
                } else if (tokens.matchToken("and")) {
                    unbounded = true;
                    tokens.requireToken("on");
                }
            }

            var intersectionSpec, mutationSpec;
            if (eventName === "intersection") {
                intersectionSpec = {};
                if (tokens.matchToken("with")) {
                    parser.requireElement("expression", tokens)
                    //Do not evaluate.
                    //intersectionSpec["with"] = parser.requireElement("expression", tokens).evaluate();
                }
                if (tokens.matchToken("having")) {
                    do {
                        if (tokens.matchToken("margin")) {
                            parser.requireElement("stringLike", tokens)
                            //intersectionSpec["rootMargin"] = parser.requireElement("stringLike", tokens).evaluate();
                        } else if (tokens.matchToken("threshold")) {
                            parser.requireElement("expression", tokens)
                            //intersectionSpec["threshold"] = parser.requireElement("expression", tokens).evaluate();
                        } else {
                            parser.raiseParseError(tokens, "Unknown intersection config specification");
                        }
                    } while (tokens.matchToken("and"));
                }
            } else if (eventName === "mutation") {
                mutationSpec = {};
                if (tokens.matchToken("of")) {
                    do {
                        if (tokens.matchToken("anything")) {
                            mutationSpec["attributes"] = true;
                            mutationSpec["subtree"] = true;
                            mutationSpec["characterData"] = true;
                            mutationSpec["childList"] = true;
                        } else if (tokens.matchToken("childList")) {
                            mutationSpec["childList"] = true;
                        } else if (tokens.matchToken("attributes")) {
                            mutationSpec["attributes"] = true;
                            mutationSpec["attributeOldValue"] = true;
                        } else if (tokens.matchToken("subtree")) {
                            mutationSpec["subtree"] = true;
                        } else if (tokens.matchToken("characterData")) {
                            mutationSpec["characterData"] = true;
                            mutationSpec["characterDataOldValue"] = true;
                        } else if (tokens.currentToken().type === "ATTRIBUTE_REF") {
                            var attribute = tokens.consumeToken();
                            if (mutationSpec["attributeFilter"] == null) {
                                mutationSpec["attributeFilter"] = [];
                            }
                            if (attribute.value.indexOf("@") == 0) {
                                mutationSpec["attributeFilter"].push(attribute.value.substring(1));
                            } else {
                                parser.raiseParseError(
                                    tokens,
                                    "Only shorthand attribute references are allowed here"
                                );
                            }
                        } else {
                            parser.raiseParseError(tokens, "Unknown mutation config specification");
                        }
                    } while (tokens.matchToken("or"));
                } else {
                    mutationSpec["attributes"] = true;
                    mutationSpec["characterData"] = true;
                    mutationSpec["childList"] = true;
                }
            }

            var from = null;
            var elsewhere = false;
            if (tokens.matchToken("from")) {
                if (tokens.matchToken("elsewhere")) {
                    elsewhere = true;
                } else {
                    tokens.pushFollow("or");
                    try {
                        from = parser.requireElement("expression", tokens)
                    } finally {
                        tokens.popFollow();
                    }
                    if (!from) {
                        parser.raiseParseError(tokens, 'Expected either target value or "elsewhere".');
                    }
                }
            }
            // support both "elsewhere" and "from elsewhere"
            if (from === null && elsewhere === false && tokens.matchToken("elsewhere")) {
                elsewhere = true;
            }

            if (tokens.matchToken("in")) {
                var inExpr = parser.parseElement('unaryExpression', tokens);
            }

            if (tokens.matchToken("debounced")) {
                tokens.requireToken("at");
                var timeExpr = parser.requireElement("unaryExpression", tokens);
                // @ts-ignore
                var debounceTime = timeExpr.evaluate({}); // OK No promise TODO make a literal time expr
            } else if (tokens.matchToken("throttled")) {
                tokens.requireToken("at");
                var timeExpr = parser.requireElement("unaryExpression", tokens);
                // @ts-ignore
                var throttleTime = timeExpr.evaluate({}); // OK No promise TODO make a literal time expr
            }

            events.push({
                execCount: 0,
                every: every,
                on: eventName,
                args: args,
                filter: filter,
                from: from,
                inExpr: inExpr,
                elsewhere: elsewhere,
                startCount: startCount,
                endCount: endCount,
                unbounded: unbounded,
                debounceTime: debounceTime,
                throttleTime: throttleTime,
                mutationSpec: mutationSpec,
                intersectionSpec: intersectionSpec,
                debounced: undefined,
                lastExec: undefined,
            });
        } while (tokens.matchToken("or"));

        var queueLast = true;
        if (!every) {
            if (tokens.matchToken("queue")) {
                if (tokens.matchToken("all")) {
                    var queueAll = true;
                    var queueLast = false;
                } else if (tokens.matchToken("first")) {
                    var queueFirst = true;
                } else if (tokens.matchToken("none")) {
                    var queueNone = true;
                } else {
                    tokens.requireToken("last");
                }
            }
        }

        var start = parser.requireElement("commandList", tokens);
        parser.ensureTerminated(start);

        var errorSymbol, errorHandler;
        if (tokens.matchToken("catch")) {
            errorSymbol = tokens.requireTokenType("IDENTIFIER").value;
            errorHandler = parser.requireElement("commandList", tokens);
            parser.ensureTerminated(errorHandler);
        }

        if (tokens.matchToken("finally")) {
            var finallyHandler = parser.requireElement("commandList", tokens);
            parser.ensureTerminated(finallyHandler);
        }

        var onFeature = {
            displayName: displayName,
            events: events,
            start: start,
            every: every,
            execCount: 0,
            errorHandler: errorHandler,
            errorSymbol: errorSymbol,
        };
        //parser.setParent(start, onFeature);
        return onFeature;
    });

    parser.addFeature("def", function (parser, runtime, tokens) {
        if (!tokens.matchToken("def")) return;
        var functionName = parser.requireElement("dotOrColonPath", tokens);
        var nameVal = functionName.evaluate(); // OK
        var nameSpace = nameVal.split(".");
        var funcName = nameSpace.pop();

        var args = [];
        if (tokens.matchOpToken("(")) {
            if (tokens.matchOpToken(")")) {
                // empty args list
            } else {
                do {
                    args.push(tokens.requireTokenType("IDENTIFIER"));
                } while (tokens.matchOpToken(","));
                tokens.requireOpToken(")");
            }
        }

        var start = parser.requireElement("commandList", tokens);

        var errorSymbol, errorHandler;
        if (tokens.matchToken("catch")) {
            errorSymbol = tokens.requireTokenType("IDENTIFIER").value;
            errorHandler = parser.parseElement("commandList", tokens);
        }

        if (tokens.matchToken("finally")) {
            var finallyHandler = parser.requireElement("commandList", tokens);
            parser.ensureTerminated(finallyHandler);
        }

        var functionFeature = {
            displayName:
                funcName +
                "(" +
                args
                    .map(function (arg) {
                        return arg.value;
                    })
                    .join(", ") +
                ")",
            name: funcName,
            args: args,
            start: start,
            errorHandler: errorHandler,
            errorSymbol: errorSymbol,
            finallyHandler: finallyHandler,
        };

        parser.ensureTerminated(start);

        // terminate error handler if any
        if (errorHandler) {
            parser.ensureTerminated(errorHandler);
        }

        //parser.setParent(start, functionFeature);
        return functionFeature;
    });

    parser.addFeature("set", function (parser, runtime, tokens) {
        let setCmd = parser.parseElement("setCommand", tokens);
        if (setCmd) {
            if (setCmd.target.scope !== "element") {
                parser.raiseParseError(tokens, "variables declared at the feature level must be element scoped.");
            }
            let setFeature = {
                start: setCmd,
            };
            parser.ensureTerminated(setCmd);
            return setFeature;
        }
    });

    parser.addFeature("init", function (parser, runtime, tokens) {
        if (!tokens.matchToken("init")) return;

        var immediately = tokens.matchToken("immediately");

        var start = parser.requireElement("commandList", tokens);
        var initFeature = {
            start: start,
        };

        // terminate body
        parser.ensureTerminated(start);
        //parser.setParent(start, initFeature);
        return initFeature;
    });

    parser.addFeature("worker", function (parser, runtime, tokens) {
        if (tokens.matchToken("worker")) {
            parser.raiseParseError(
                tokens,
                "In order to use the 'worker' feature, include " +
                "the _hyperscript worker plugin. See " +
                "https://hyperscript.org/features/worker/ for " +
                "more info."
            );
            return undefined
        }
    });

    parser.addFeature("behavior", function (parser, runtime, tokens) {
        if (!tokens.matchToken("behavior")) return;
        var path = parser.requireElement("dotOrColonPath", tokens).evaluate();
        var nameSpace = path.split(".");
        var name = nameSpace.pop();

        var formalParams = [];
        if (tokens.matchOpToken("(") && !tokens.matchOpToken(")")) {
            do {
                formalParams.push(tokens.requireTokenType("IDENTIFIER").value);
            } while (tokens.matchOpToken(","));
            tokens.requireOpToken(")");
        }
        var hs = parser.requireElement("hyperscript", tokens);
        for (var i = 0; i < hs.features.length; i++) {
            var feature = hs.features[i];
            feature.behavior = path;
        }

        return {};
    });

    parser.addFeature("install", function (parser, runtime, tokens) {
        if (!tokens.matchToken("install")) return;
        var behaviorPath = parser.requireElement("dotOrColonPath", tokens).evaluate();
        var behaviorNamespace = behaviorPath.split(".");
        var args = parser.parseElement("namedArgumentList", tokens);

        var installFeature;
        return (installFeature = {});
    });

    parser.addGrammarElement("jsBody", function (parser, runtime, tokens) {
        var jsSourceStart = tokens.currentToken().start;
        var jsLastToken = tokens.currentToken();

        var funcNames = [];
        var funcName = "";
        var expectFunctionDeclaration = false;
        while (tokens.hasMore()) {
            jsLastToken = tokens.consumeToken();
            var peek = tokens.token(0, true);
            if (peek.type === "IDENTIFIER" && peek.value === "end") {
                break;
            }
            if (expectFunctionDeclaration) {
                if (jsLastToken.type === "IDENTIFIER" || jsLastToken.type === "NUMBER") {
                    funcName += jsLastToken.value;
                } else {
                    if (funcName !== "") funcNames.push(funcName);
                    funcName = "";
                    expectFunctionDeclaration = false;
                }
            } else if (jsLastToken.type === "IDENTIFIER" && jsLastToken.value === "function") {
                expectFunctionDeclaration = true;
            }
        }
        var jsSourceEnd = jsLastToken.end + 1;

        return {
            type: "jsBody",
            exposedFunctionNames: funcNames,
            jsSource: tokens.source.substring(jsSourceStart, jsSourceEnd),
        };
    });

    parser.addFeature("js", function (parser, runtime, tokens) {
        if (!tokens.matchToken("js")) return;
        var jsBody = parser.requireElement("jsBody", tokens);

        var jsSource =
            jsBody.jsSource +
            "\nreturn { " +
            jsBody.exposedFunctionNames
                .map(function (name) {
                    return name + ":" + name;
                })
                .join(",") +
            " } ";
        var func = new Function(jsSource);

        return {
            jsSource: jsSource,
            function: func,
            exposedFunctionNames: jsBody.exposedFunctionNames,
        };
    });

    parser.addCommand("js", function (parser, runtime, tokens) {
        if (!tokens.matchToken("js")) return;
        // Parse inputs
        var inputs = [];
        if (tokens.matchOpToken("(")) {
            if (tokens.matchOpToken(")")) {
                // empty input list
            } else {
                do {
                    var inp = tokens.requireTokenType("IDENTIFIER");
                    inputs.push(inp.value);
                } while (tokens.matchOpToken(","));
                tokens.requireOpToken(")");
            }
        }

        var jsBody = parser.requireElement("jsBody", tokens);
        tokens.matchToken("end");

        var func = varargConstructor(Function, inputs.concat([jsBody.jsSource]));

        var command = {
            jsSource: jsBody.jsSource,
            function: func,
            inputs: inputs,
        };
        return command;
    });

    parser.addCommand("async", function (parser, runtime, tokens) {
        if (!tokens.matchToken("async")) return;
        if (tokens.matchToken("do")) {
            var body = parser.requireElement("commandList", tokens);

            // Append halt
            var end = body;
            while (end.next) end = end.next;
            end.next = HALT;

            tokens.requireToken("end");
        } else {
            var body = parser.requireElement("command", tokens);
        }
        var command = {
            body: body,
        };
        //parser.setParent(body, command);
        return command;
    });

    parser.addCommand("tell", function (parser, runtime, tokens) {
        var startToken = tokens.currentToken();
        if (!tokens.matchToken("tell")) return;
        var value = parser.requireElement("expression", tokens);
        var body = parser.requireElement("commandList", tokens);
        if (tokens.hasMore() && !parser.featureStart(tokens.currentToken())) {
            tokens.requireToken("end");
        }
        var slot = "tell_" + startToken.start;
        var tellCmd = {
            value: value,
            body: body,
            args: [value],
            resolveNext: function (context) {
                throwOnlyParsingIsSupported()
            },
        };
        //parser.setParent(body, tellCmd);
        return tellCmd;
    });

    parser.addCommand("wait", function (parser, runtime, tokens) {
        if (!tokens.matchToken("wait")) return;
        var command;

        // wait on event
        if (tokens.matchToken("for")) {
            tokens.matchToken("a"); // optional "a"
            var events = [];
            do {
                var lookahead = tokens.token(0);
                if (lookahead.type === 'NUMBER' || lookahead.type === 'L_PAREN') {
                    parser.requireElement('expression', tokens)

                    events.push({
                        time: -1
                        //Do not evaluate.
                        //time: parser.requireElement('expression', tokens).evaluate() // TODO: do we want to allow async here?
                    })
                } else {
                    events.push({
                        name: parser.requireElement("dotOrColonPath", tokens, "Expected event name").evaluate(),
                        args: parseEventArgs(tokens),
                    });
                }
            } while (tokens.matchToken("or"));

            if (tokens.matchToken("from")) {
                var on = parser.requireElement("expression", tokens);
            }

            // wait on event
            command = {
                event: events,
                on: on,
                args: [on],
            };
            return command;
        } else {
            var time;
            if (tokens.matchToken("a")) {
                tokens.requireToken("tick");
                time = 0;
            } else {
                time = parser.requireElement("expression", tokens);
            }

            command = {
                type: "waitCmd",
                time: time,
                args: [time],
            };
            return command;
        }
    });

    // TODO  - colon path needs to eventually become part of ruby-style symbols
    parser.addGrammarElement("dotOrColonPath", function (parser, runtime, tokens) {
        var root = tokens.matchTokenType("IDENTIFIER");
        if (root) {
            var path = [root.value];

            var separator = tokens.matchOpToken(".") || tokens.matchOpToken(":");
            if (separator) {
                do {
                    path.push(tokens.requireTokenType("IDENTIFIER", "NUMBER").value);
                } while (tokens.matchOpToken(separator.value));
            }

            return {
                type: "dotOrColonPath",
                path: path,
                evaluate: function () {
                    return path.join(separator ? separator.value : "");
                },
            };
        }
    });


    parser.addGrammarElement("eventName", function (parser, runtime, tokens) {
        var token;
        if ((token = tokens.matchTokenType("STRING"))) {
            return {
                evaluate: function () {
                    return token.value;
                },
            };
        }

        return parser.parseElement("dotOrColonPath", tokens);
    });

    function parseSendCmd(cmdType, parser, runtime, tokens) {
        var eventName = parser.requireElement("eventName", tokens);

        var details = parser.parseElement("namedArgumentList", tokens);
        if ((cmdType === "send" && tokens.matchToken("to")) ||
            (cmdType === "trigger" && tokens.matchToken("on"))) {
            var toExpr = parser.requireElement("expression", tokens);
        } else {
            var toExpr = parser.requireElement("implicitMeTarget", tokens);
        }

        var sendCmd = {
            eventName: eventName,
            details: details,
            to: toExpr,
            args: [toExpr, eventName, details],
        };
        return sendCmd;
    }

    parser.addCommand("trigger", function (parser, runtime, tokens) {
        if (tokens.matchToken("trigger")) {
            return parseSendCmd("trigger", parser, runtime, tokens);
        }
    });

    parser.addCommand("send", function (parser, runtime, tokens) {
        if (tokens.matchToken("send")) {
            return parseSendCmd("send", parser, runtime, tokens);
        }
    });

    var parseReturnFunction = function (parser, runtime, tokens, returnAValue) {
        if (returnAValue) {
            if (parser.commandBoundary(tokens.currentToken())) {
                parser.raiseParseError(tokens, "'return' commands must return a value.  If you do not wish to return a value, use 'exit' instead.");
            } else {
                var value = parser.requireElement("expression", tokens);
            }
        }

        var returnCmd = {
            value: value,
            args: [value],
        };
        return returnCmd;
    };

    parser.addCommand("return", function (parser, runtime, tokens) {
        if (tokens.matchToken("return")) {
            return parseReturnFunction(parser, runtime, tokens, true);
        }
    });

    parser.addCommand("exit", function (parser, runtime, tokens) {
        if (tokens.matchToken("exit")) {
            return parseReturnFunction(parser, runtime, tokens, false);
        }
    });

    parser.addCommand("halt", function (parser, runtime, tokens) {
        if (tokens.matchToken("halt")) {
            if (tokens.matchToken("the")) {
                tokens.requireToken("event");
                // optional possessive
                if (tokens.matchOpToken("'")) {
                    tokens.requireToken("s");
                }
                var keepExecuting = true;
            }
            if (tokens.matchToken("bubbling")) {
                var bubbling = true;
            } else if (tokens.matchToken("default")) {
                var haltDefault = true;
            }
            var exit = parseReturnFunction(parser, runtime, tokens, false);

            var haltCmd = {
                keepExecuting: true,
                bubbling: bubbling,
                haltDefault: haltDefault,
                exit: exit,
            };
            return haltCmd;
        }
    });

    parser.addCommand("log", function (parser, runtime, tokens) {
        if (!tokens.matchToken("log")) return;
        var exprs = [parser.parseElement("expression", tokens)];
        while (tokens.matchOpToken(",")) {
            exprs.push(parser.requireElement("expression", tokens));
        }
        if (tokens.matchToken("with")) {
            var withExpr = parser.requireElement("expression", tokens);
        }
        var logCmd = {
            exprs: exprs,
            withExpr: withExpr,
            args: [withExpr, exprs],
        };
        return logCmd;
    });

    parser.addCommand("beep!", function (parser, runtime, tokens) {
        if (!tokens.matchToken("beep!")) return;
        var exprs = [parser.parseElement("expression", tokens)];
        while (tokens.matchOpToken(",")) {
            exprs.push(parser.requireElement("expression", tokens));
        }
        var beepCmd = {
            exprs: exprs,
            args: [exprs],
        };
        return beepCmd;
    });

    parser.addCommand("throw", function (parser, runtime, tokens) {
        if (!tokens.matchToken("throw")) return;
        var expr = parser.requireElement("expression", tokens);
        var throwCmd = {
            expr: expr,
            args: [expr],
        };
        return throwCmd;
    });

    var parseCallOrGet = function (parser, runtime, tokens) {
        var expr = parser.requireElement("expression", tokens);
        var callCmd = {
            expr: expr,
            args: [expr],
        };
        return callCmd;
    };
    parser.addCommand("call", function (parser, runtime, tokens) {
        if (!tokens.matchToken("call")) return;
        var call = parseCallOrGet(parser, runtime, tokens);
        if (call.expr && call.expr.type !== "functionCall") {
            parser.raiseParseError(tokens, "Must be a function invocation");
        }
        return call;
    });
    parser.addCommand("get", function (parser, runtime, tokens) {
        if (tokens.matchToken("get")) {
            return parseCallOrGet(parser, runtime, tokens);
        }
    });

    parser.addCommand("make", function (parser, runtime, tokens) {
        if (!tokens.matchToken("make")) return;
        tokens.matchToken("a") || tokens.matchToken("an");

        var expr = parser.requireElement("expression", tokens);

        var args = [];
        if (expr.type !== "queryRef" && tokens.matchToken("from")) {
            do {
                args.push(parser.requireElement("expression", tokens));
            } while (tokens.matchOpToken(","));
        }

        if (tokens.matchToken("called")) {
            var target = parser.requireElement("symbol", tokens);
        }

        var command;
        if (expr.type === "queryRef") {
            command = {
            };
            return command;
        } else {
            command = {
                args: [expr, args],
            };
            return command;
        }
    });

    parser.addGrammarElement("pseudoCommand", function (parser, runtime, tokens) {

        let lookAhead = tokens.token(1);
        if (!(lookAhead && lookAhead.op && (lookAhead.value === '.' || lookAhead.value === "("))) {
            return null;
        }

        var expr = parser.requireElement("primaryExpression", tokens);

        var rootRoot = expr.root;
        var root = expr;
        while (rootRoot.root != null) {
            root = root.root;
            rootRoot = rootRoot.root;
        }

        if (expr.type !== "functionCall") {
            parser.raiseParseError(tokens, "Pseudo-commands must be function calls");
        }

        if (root.type === "functionCall" && root.root.root == null) {
            if (tokens.matchAnyToken("the", "to", "on", "with", "into", "from", "at")) {
                var realRoot = parser.requireElement("expression", tokens);
            } else if (tokens.matchToken("me")) {
                var realRoot = parser.requireElement("implicitMeTarget", tokens);
            }
        }

        /** @type {ASTNode} */

        var pseudoCommand
        if (realRoot) {
            pseudoCommand = {
                type: "pseudoCommand",
                root: realRoot,
                argExressions: root.argExressions,
                args: [realRoot, root.argExressions],
            }
        } else {
            pseudoCommand = {
                type: "pseudoCommand",
                expr: expr,
                args: [expr],
            };
        }

        return pseudoCommand;
    });

    /**
    * @param {Parser} parser
    * @param {Runtime} runtime
    * @param {Tokens} tokens
    * @param {*} target
    * @param {*} value
    * @returns
    */
    var makeSetter = function (parser, runtime, tokens, target, value) {

        var symbolWrite = target.type === "symbol";
        var attributeWrite = target.type === "attributeRef";
        var styleWrite = target.type === "styleRef";
        var arrayWrite = target.type === "arrayIndex";

        if (!(attributeWrite || styleWrite || symbolWrite) && target.root == null) {
            parser.raiseParseError(tokens, "Can only put directly into symbols, not references");
        }

        var rootElt = null;
        var prop = null;
        if (symbolWrite) {
            // rootElt is null
        } else if (attributeWrite || styleWrite) {
            rootElt = parser.requireElement("implicitMeTarget", tokens);
            var attribute = target;
        } else if (arrayWrite) {
            prop = target.firstIndex;
            rootElt = target.root;
        } else {
            prop = target.prop ? target.prop.value : null;
            var attribute = target.attribute;
            rootElt = target.root;
        }

        /** @type {ASTNode} */
        var setCmd = {
            target: target,
            symbolWrite: symbolWrite,
            value: value,
            args: [rootElt, prop, value],
        };
        return setCmd;
    };

    parser.addCommand("default", function (parser, runtime, tokens) {
        if (!tokens.matchToken("default")) return;
        var target = parser.requireElement("assignableExpression", tokens);
        tokens.requireToken("to");

        var value = parser.requireElement("expression", tokens);

        /** @type {ASTNode} */
        var setter = makeSetter(parser, runtime, tokens, target, value);
        var defaultCmd = {
            target: target,
            value: value,
            setter: setter,
            args: [target],
        };
        setter.parent = defaultCmd;
        return defaultCmd;
    });

    parser.addCommand("set", function (parser, runtime, tokens) {
        if (!tokens.matchToken("set")) return;
        if (tokens.currentToken().type === "L_BRACE") {
            var obj = parser.requireElement("objectLiteral", tokens);
            tokens.requireToken("on");
            var target = parser.requireElement("expression", tokens);

            var command = {
                objectLiteral: obj,
                target: target,
                args: [obj, target],
            };
            return command;
        }

        try {
            tokens.pushFollow("to");
            var target = parser.requireElement("assignableExpression", tokens);
        } finally {
            tokens.popFollow();
        }
        tokens.requireToken("to");
        var value = parser.requireElement("expression", tokens);
        return makeSetter(parser, runtime, tokens, target, value);
    });

    parser.addCommand("if", function (parser, runtime, tokens) {
        if (!tokens.matchToken("if")) return;
        var expr = parser.requireElement("expression", tokens);
        tokens.matchToken("then"); // optional 'then'
        var trueBranch = parser.parseElement("commandList", tokens);
        var nestedIfStmt = false;
        let elseToken = tokens.matchToken("else") || tokens.matchToken("otherwise");
        if (elseToken) {
            let elseIfIfToken = tokens.peekToken("if");
            nestedIfStmt = elseIfIfToken != null && elseIfIfToken.line === elseToken.line;
            if (nestedIfStmt) {
                var falseBranch = parser.parseElement("command", tokens);
            } else {
                var falseBranch = parser.parseElement("commandList", tokens);
            }
        }
        if (tokens.hasMore() && !nestedIfStmt) {
            tokens.requireToken("end");
        }

        /** @type {ASTNode} */
        var ifCmd = {
            expr: expr,
            trueBranch: trueBranch,
            falseBranch: falseBranch,
            args: [expr],
        };
        //parser.setParent(trueBranch, ifCmd);
        //parser.setParent(falseBranch, ifCmd);
        return ifCmd;
    });

    var parseRepeatExpression = function (parser, tokens, runtime, startedWithForToken) {
        var innerStartToken = tokens.currentToken();
        var identifier;
        if (tokens.matchToken("for") || startedWithForToken) {
            var identifierToken = tokens.requireTokenType("IDENTIFIER");
            identifier = identifierToken.value;
            tokens.requireToken("in");
            var expression = parser.requireElement("expression", tokens);
        } else if (tokens.matchToken("in")) {
            identifier = "it";
            var expression = parser.requireElement("expression", tokens);
        } else if (tokens.matchToken("while")) {
            var whileExpr = parser.requireElement("expression", tokens);
        } else if (tokens.matchToken("until")) {
            var isUntil = true;
            if (tokens.matchToken("event")) {
                var evt = parser.requireElement("dotOrColonPath", tokens, "Expected event name");
                if (tokens.matchToken("from")) {
                    var on = parser.requireElement("expression", tokens);
                }
            } else {
                var whileExpr = parser.requireElement("expression", tokens);
            }
        } else {
            if (!parser.commandBoundary(tokens.currentToken()) &&
                tokens.currentToken().value !== 'forever') {
                var times = parser.requireElement("expression", tokens);
                tokens.requireToken("times");
            } else {
                tokens.matchToken("forever"); // consume optional forever
                var forever = true;
            }
        }

        if (tokens.matchToken("index")) {
            var identifierToken = tokens.requireTokenType("IDENTIFIER");
            var indexIdentifier = identifierToken.value;
        }

        var loop = parser.parseElement("commandList", tokens);
        if (loop && evt) {
            // if this is an event based loop, wait a tick at the end of the loop so that
            // events have a chance to trigger in the loop condition o_O)))
            var last = loop;
            while (last.next) {
                last = last.next;
            }
            var waitATick = {
                type: "waitATick",
            };
            last.next = waitATick;
        }
        if (tokens.hasMore()) {
            tokens.requireToken("end");
        }

        if (identifier == null) {
            identifier = "_implicit_repeat_" + innerStartToken.start;
            var slot = identifier;
        } else {
            var slot = identifier + "_" + innerStartToken.start;
        }

        var repeatCmd = {
            identifier: identifier,
            indexIdentifier: indexIdentifier,
            slot: slot,
            expression: expression,
            forever: forever,
            times: times,
            until: isUntil,
            event: evt,
            on: on,
            whileExpr: whileExpr,
            resolveNext: function () {
                return this;
            },
            loop: loop,
            args: [whileExpr, times],
        };
        //parser.setParent(loop, repeatCmd);
        var repeatInit = {
            name: "repeatInit",
            args: [expression, evt, on],
        };
        //parser.setParent(repeatCmd, repeatInit);
        return repeatInit;
    };

    parser.addCommand("repeat", function (parser, runtime, tokens) {
        if (tokens.matchToken("repeat")) {
            return parseRepeatExpression(parser, tokens, runtime, false);
        }
    });

    parser.addCommand("for", function (parser, runtime, tokens) {
        if (tokens.matchToken("for")) {
            return parseRepeatExpression(parser, tokens, runtime, true);
        }
    });

    parser.addCommand("continue", function (parser, runtime, tokens) {

        if (!tokens.matchToken("continue")) return;

        var command = {
        };
        return command;
    });

    parser.addCommand("break", function (parser, runtime, tokens) {

        if (!tokens.matchToken("break")) return;

        var command = {
        };
        return command;
    });

    parser.addGrammarElement("stringLike", function (parser, runtime, tokens) {
        return parser.parseAnyOf(["string", "nakedString"], tokens);
    });

    parser.addCommand("append", function (parser, runtime, tokens) {
        if (!tokens.matchToken("append")) return;
        var targetExpr = null;

        var value = parser.requireElement("expression", tokens);

        /** @type {ASTNode} */
        var implicitResultSymbol = {
            type: "symbol",
            evaluate: function (context) {
                throwOnlyParsingIsSupported()
            },
        };

        if (tokens.matchToken("to")) {
            targetExpr = parser.requireElement("expression", tokens);
        } else {
            targetExpr = implicitResultSymbol;
        }

        var setter = null;
        if (targetExpr.type === "symbol" || targetExpr.type === "attributeRef" || targetExpr.root != null) {
            setter = makeSetter(parser, runtime, tokens, targetExpr, implicitResultSymbol);
        }

        var command = {
            value: value,
            target: targetExpr,
            args: [targetExpr, value],
        };

        if (setter != null) {
            setter.parent = command;
        }

        return command;
    });

    function parsePickRange(parser, runtime, tokens) {
        tokens.matchToken("at") || tokens.matchToken("from");
        const rv = { includeStart: true, includeEnd: false }

        rv.from = tokens.matchToken("start") ? 0 : parser.requireElement("expression", tokens)

        if (tokens.matchToken("to") || tokens.matchOpToken("..")) {
            if (tokens.matchToken("end")) {
                rv.toEnd = true;
            } else {
                rv.to = parser.requireElement("expression", tokens);
            }
        }

        if (tokens.matchToken("inclusive")) rv.includeEnd = true;
        else if (tokens.matchToken("exclusive")) rv.includeStart = false;

        return rv;
    }

    class RegExpIterator {
        constructor(re, str) {
            this.re = re;
            this.str = str;
        }

        next() {
            const match = this.re.exec(this.str);
            if (match === null) return { done: true };
            else return { value: match };
        }
    }

    class RegExpIterable {
        constructor(re, flags, str) {
            this.re = re;
            this.flags = flags;
            this.str = str;
        }

        [Symbol.iterator]() {
            return new RegExpIterator(new RegExp(this.re, this.flags), this.str);
        }
    }

    parser.addCommand("pick", (parser, runtime, tokens) => {
        if (!tokens.matchToken("pick")) return;

        tokens.matchToken("the");

        if (tokens.matchToken("item") || tokens.matchToken("items")
            || tokens.matchToken("character") || tokens.matchToken("characters")) {
            const range = parsePickRange(parser, runtime, tokens);

            tokens.requireToken("from");
            const root = parser.requireElement("expression", tokens);

            return {
                args: [root, range.from, range.to],
                op(ctx, root, from, to) {
                    throwOnlyParsingIsSupported()
                }
            }
        }

        if (tokens.matchToken("match")) {
            tokens.matchToken("of");
            const re = parser.parseElement("expression", tokens);
            let flags = ""
            if (tokens.matchOpToken("|")) {
                flags = tokens.requireToken("identifier").value;
            }

            tokens.requireToken("from");
            const root = parser.parseElement("expression", tokens);

            return {
                args: [root, re],
                op(ctx, root, re) {
                    throwOnlyParsingIsSupported()
                }
            }
        }

        if (tokens.matchToken("matches")) {
            tokens.matchToken("of");
            const re = parser.parseElement("expression", tokens);
            let flags = "gu"
            if (tokens.matchOpToken("|")) {
                flags = 'g' + tokens.requireToken("identifier").value.replace('g', '');
            }
            console.log('flags', flags)

            tokens.requireToken("from");
            const root = parser.parseElement("expression", tokens);

            return {
                args: [root, re],
                op(ctx, root, re) {
                    throwOnlyParsingIsSupported()
                }
            }
        }
    });

    parser.addCommand("increment", function (parser, runtime, tokens) {
        if (!tokens.matchToken("increment")) return;
        var amountExpr;

        // This is optional.  Defaults to "result"
        var target = parser.parseElement("assignableExpression", tokens);

        // This is optional. Defaults to 1.
        if (tokens.matchToken("by")) {
            amountExpr = parser.requireElement("expression", tokens);
        }

        var implicitIncrementOp = {
            type: "implicitIncrementOp",
            target: target,
            args: [target, amountExpr],
            evaluate: function (context) {
                throwOnlyParsingIsSupported()
            }
        };

        return makeSetter(parser, runtime, tokens, target, implicitIncrementOp);
    });

    parser.addCommand("decrement", function (parser, runtime, tokens) {
        if (!tokens.matchToken("decrement")) return;
        var amountExpr;

        // This is optional.  Defaults to "result"
        var target = parser.parseElement("assignableExpression", tokens);

        // This is optional. Defaults to 1.
        if (tokens.matchToken("by")) {
            amountExpr = parser.requireElement("expression", tokens);
        }

        var implicitDecrementOp = {
            type: "implicitDecrementOp",
            target: target,
            args: [target, amountExpr],
            evaluate: function (context) {
                throwOnlyParsingIsSupported()
            }
        };

        return makeSetter(parser, runtime, tokens, target, implicitDecrementOp);
    });

    function parseConversionInfo(tokens, parser) {
        var type = "text";
        var conversion;
        tokens.matchToken("a") || tokens.matchToken("an");
        if (tokens.matchToken("json") || tokens.matchToken("Object")) {
            type = "json";
        } else if (tokens.matchToken("response")) {
            type = "response";
        } else if (tokens.matchToken("html")) {
            type = "html";
        } else if (tokens.matchToken("text")) {
            // default, ignore
        } else {
            conversion = parser.requireElement("dotOrColonPath", tokens).evaluate();
        }
        return { type, conversion };
    }

    parser.addCommand("fetch", function (parser, runtime, tokens) {
        if (!tokens.matchToken("fetch")) return;
        var url = parser.requireElement("stringLike", tokens);

        if (tokens.matchToken("as")) {
            var conversionInfo = parseConversionInfo(tokens, parser);
        }

        if (tokens.matchToken("with") && tokens.currentToken().value !== "{") {
            var args = parser.parseElement("nakedNamedArgumentList", tokens);
        } else {
            var args = parser.parseElement("objectLiteral", tokens);
        }

        if (conversionInfo == null && tokens.matchToken("as")) {
            conversionInfo = parseConversionInfo(tokens, parser);
        }

        var type = conversionInfo ? conversionInfo.type : "text";
        var conversion = conversionInfo ? conversionInfo.conversion : null

        /** @type {ASTNode} */
        var fetchCmd = {
            url: url,
            argExpressions: args,
            args: [url, args],
        };
        return fetchCmd;
    });
}

function hyperscriptWebGrammar(parser) {
    parser.addCommand("settle", function (parser, runtime, tokens) {
        if (tokens.matchToken("settle")) {
            if (!parser.commandBoundary(tokens.currentToken())) {
                var onExpr = parser.requireElement("expression", tokens);
            } else {
                var onExpr = parser.requireElement("implicitMeTarget", tokens);
            }

            var settleCommand = {
                type: "settleCmd",
                args: [onExpr],
            };
            return settleCommand;
        }
    });

    parser.addCommand("add", function (parser, runtime, tokens) {
        if (tokens.matchToken("add")) {
            var classRef = parser.parseElement("classRef", tokens);
            var attributeRef = null;
            var cssDeclaration = null;
            if (classRef == null) {
                attributeRef = parser.parseElement("attributeRef", tokens);
                if (attributeRef == null) {
                    cssDeclaration = parser.parseElement("styleLiteral", tokens);
                    if (cssDeclaration == null) {
                        parser.raiseParseError(tokens, "Expected either a class reference or attribute expression");
                    }
                }
            } else {
                var classRefs = [classRef];
                while ((classRef = parser.parseElement("classRef", tokens))) {
                    classRefs.push(classRef);
                }
            }

            if (tokens.matchToken("to")) {
                var toExpr = parser.requireElement("expression", tokens);
            } else {
                var toExpr = parser.requireElement("implicitMeTarget", tokens);
            }

            if (tokens.matchToken("when")) {
                if (cssDeclaration) {
                    parser.raiseParseError(tokens, "Only class and properties are supported with a when clause")
                }
                var when = parser.requireElement("expression", tokens);
            }

            if (classRefs) {
                return {
                    classRefs: classRefs,
                    to: toExpr,
                    args: [toExpr, classRefs],
                };
            } else if (attributeRef) {
                return {
                    type: "addCmd",
                    attributeRef: attributeRef,
                    to: toExpr,
                    args: [toExpr],
                };
            } else {
                return {
                    type: "addCmd",
                    cssDeclaration: cssDeclaration,
                    to: toExpr,
                    args: [toExpr, cssDeclaration],
                };
            }
        }
    });

    parser.addGrammarElement("styleLiteral", function (parser, runtime, tokens) {
        if (!tokens.matchOpToken("{")) return;

        var stringParts = [""]
        var exprs = []

        while (tokens.hasMore()) {
            if (tokens.matchOpToken("\\")) {
                tokens.consumeToken();
            } else if (tokens.matchOpToken("}")) {
                break;
            } else if (tokens.matchToken("$")) {
                var opencurly = tokens.matchOpToken("{");
                var expr = parser.parseElement("expression", tokens);
                if (opencurly) tokens.requireOpToken("}");

                exprs.push(expr)
                stringParts.push("")
            } else {
                var tok = tokens.consumeToken();
                stringParts[stringParts.length - 1] += tokens.source.substring(tok.start, tok.end);
            }

            stringParts[stringParts.length - 1] += tokens.lastWhitespace();
        }

        return {
            type: "styleLiteral",
            args: [exprs],
            evaluate: function (ctx) {
                throwOnlyParsingIsSupported()
            }
        }
    })

    parser.addCommand("remove", function (parser, runtime, tokens) {
        if (tokens.matchToken("remove")) {
            var classRef = parser.parseElement("classRef", tokens);
            var attributeRef = null;
            var elementExpr = null;
            if (classRef == null) {
                attributeRef = parser.parseElement("attributeRef", tokens);
                if (attributeRef == null) {
                    elementExpr = parser.parseElement("expression", tokens);
                    if (elementExpr == null) {
                        parser.raiseParseError(
                            tokens,
                            "Expected either a class reference, attribute expression or value expression"
                        );
                    }
                }
            } else {
                var classRefs = [classRef];
                while ((classRef = parser.parseElement("classRef", tokens))) {
                    classRefs.push(classRef);
                }
            }

            if (tokens.matchToken("from")) {
                var fromExpr = parser.requireElement("expression", tokens);
            } else {
                if (elementExpr == null) {
                    var fromExpr = parser.requireElement("implicitMeTarget", tokens);
                }
            }

            if (elementExpr) {
                return {
                    elementExpr: elementExpr,
                    from: fromExpr,
                    args: [elementExpr, fromExpr],
                };
            } else {
                return {
                    classRefs: classRefs,
                    attributeRef: attributeRef,
                    elementExpr: elementExpr,
                    from: fromExpr,
                    args: [classRefs, fromExpr],
                };
            }
        }
    });

    parser.addCommand("toggle", function (parser, runtime, tokens) {
        if (tokens.matchToken("toggle")) {
            tokens.matchAnyToken("the", "my");
            if (tokens.currentToken().type === "STYLE_REF") {
                let styleRef = tokens.consumeToken();
                var name = styleRef.value.substr(1);
                var visibility = true;
                var hideShowStrategy = resolveHideShowStrategy(parser, tokens, name);
                if (tokens.matchToken("of")) {
                    tokens.pushFollow("with");
                    try {
                        var onExpr = parser.requireElement("expression", tokens);
                    } finally {
                        tokens.popFollow();
                    }
                } else {
                    var onExpr = parser.requireElement("implicitMeTarget", tokens);
                }
            } else if (tokens.matchToken("between")) {
                var between = true;
                var classRef = parser.parseElement("classRef", tokens);
                tokens.requireToken("and");
                var classRef2 = parser.requireElement("classRef", tokens);
            } else {
                var classRef = parser.parseElement("classRef", tokens);
                var attributeRef = null;
                if (classRef == null) {
                    attributeRef = parser.parseElement("attributeRef", tokens);
                    if (attributeRef == null) {
                        parser.raiseParseError(tokens, "Expected either a class reference or attribute expression");
                    }
                } else {
                    var classRefs = [classRef];
                    while ((classRef = parser.parseElement("classRef", tokens))) {
                        classRefs.push(classRef);
                    }
                }
            }

            if (visibility !== true) {
                if (tokens.matchToken("on")) {
                    var onExpr = parser.requireElement("expression", tokens);
                } else {
                    var onExpr = parser.requireElement("implicitMeTarget", tokens);
                }
            }

            if (tokens.matchToken("for")) {
                var time = parser.requireElement("expression", tokens);
            } else if (tokens.matchToken("until")) {
                var evt = parser.requireElement("dotOrColonPath", tokens, "Expected event name");
                if (tokens.matchToken("from")) {
                    var from = parser.requireElement("expression", tokens);
                }
            }

            var toggleCmd = {
                classRef: classRef,
                classRef2: classRef2,
                classRefs: classRefs,
                attributeRef: attributeRef,
                on: onExpr,
                time: time,
                evt: evt,
                from: from,
                toggle: function (on, classRef, classRef2, classRefs) {
                    runtime.nullCheck(on, onExpr);
                    if (visibility) {
                        runtime.implicitLoop(on, function (target) {
                            hideShowStrategy("toggle", target);
                        });
                    } else if (between) {
                        runtime.implicitLoop(on, function (target) {
                            if (target.classList.contains(classRef.className)) {
                                target.classList.remove(classRef.className);
                                target.classList.add(classRef2.className);
                            } else {
                                target.classList.add(classRef.className);
                                target.classList.remove(classRef2.className);
                            }
                        });
                    } else if (classRefs) {
                        runtime.forEach(classRefs, function (classRef) {
                            runtime.implicitLoop(on, function (target) {
                                target.classList.toggle(classRef.className);
                            });
                        });
                    } else {
                        runtime.forEach(on, function (target) {
                            if (target.hasAttribute(attributeRef.name)) {
                                target.removeAttribute(attributeRef.name);
                            } else {
                                target.setAttribute(attributeRef.name, attributeRef.value);
                            }
                        });
                    }
                },
                args: [onExpr, time, evt, from, classRef, classRef2, classRefs],
            };
            return toggleCmd;
        }
    });

    var HIDE_SHOW_STRATEGIES = {
        display: function (op, element, arg) {
            if (arg) {
                element.style.display = arg;
            } else if (op === "toggle") {
                if (getComputedStyle(element).display === "none") {
                    HIDE_SHOW_STRATEGIES.display("show", element, arg);
                } else {
                    HIDE_SHOW_STRATEGIES.display("hide", element, arg);
                }
            } else if (op === "hide") {
                const internalData = parser.runtime.getInternalData(element);
                if (internalData.originalDisplay == null) {
                    internalData.originalDisplay = element.style.display;
                }
                element.style.display = "none";
            } else {
                const internalData = parser.runtime.getInternalData(element);
                if (internalData.originalDisplay && internalData.originalDisplay !== 'none') {
                    element.style.display = internalData.originalDisplay;
                } else {
                    element.style.removeProperty('display');
                }
            }
        },
        visibility: function (op, element, arg) {
            if (arg) {
                element.style.visibility = arg;
            } else if (op === "toggle") {
                if (getComputedStyle(element).visibility === "hidden") {
                    HIDE_SHOW_STRATEGIES.visibility("show", element, arg);
                } else {
                    HIDE_SHOW_STRATEGIES.visibility("hide", element, arg);
                }
            } else if (op === "hide") {
                element.style.visibility = "hidden";
            } else {
                element.style.visibility = "visible";
            }
        },
        opacity: function (op, element, arg) {
            if (arg) {
                element.style.opacity = arg;
            } else if (op === "toggle") {
                if (getComputedStyle(element).opacity === "0") {
                    HIDE_SHOW_STRATEGIES.opacity("show", element, arg);
                } else {
                    HIDE_SHOW_STRATEGIES.opacity("hide", element, arg);
                }
            } else if (op === "hide") {
                element.style.opacity = "0";
            } else {
                element.style.opacity = "1";
            }
        },
    };

    var parseShowHideTarget = function (parser, runtime, tokens) {
        var target;
        var currentTokenValue = tokens.currentToken();
        if (currentTokenValue.value === "when" || currentTokenValue.value === "with" || parser.commandBoundary(currentTokenValue)) {
            target = parser.parseElement("implicitMeTarget", tokens);
        } else {
            target = parser.parseElement("expression", tokens);
        }
        return target;
    };

    var resolveHideShowStrategy = function (parser, tokens, name) {
        var configDefault = config.defaultHideShowStrategy;
        var strategies = HIDE_SHOW_STRATEGIES;
        if (config.hideShowStrategies) {
            strategies = Object.assign(strategies, config.hideShowStrategies); // merge in user provided strategies
        }
        name = name || configDefault || "display";
        var value = strategies[name];
        if (value == null) {
            parser.raiseParseError(tokens, "Unknown show/hide strategy : " + name);
        }
        return value;
    };

    parser.addCommand("hide", function (parser, runtime, tokens) {
        if (tokens.matchToken("hide")) {
            var targetExpr = parseShowHideTarget(parser, runtime, tokens);

            var name = null;
            if (tokens.matchToken("with")) {
                name = tokens.requireTokenType("IDENTIFIER", "STYLE_REF").value;
                if (name.indexOf("*") === 0) {
                    name = name.substr(1);
                }
            }
            var hideShowStrategy = resolveHideShowStrategy(parser, tokens, name);

            return {
                target: targetExpr,
                args: [targetExpr],
            };
        }
    });

    parser.addCommand("show", function (parser, runtime, tokens) {
        if (tokens.matchToken("show")) {
            var targetExpr = parseShowHideTarget(parser, runtime, tokens);

            var name = null;
            if (tokens.matchToken("with")) {
                name = tokens.requireTokenType("IDENTIFIER", "STYLE_REF").value;
                if (name.indexOf("*") === 0) {
                    name = name.substr(1);
                }
            }
            var arg = null;
            if (tokens.matchOpToken(":")) {
                var tokenArr = tokens.consumeUntilWhitespace();
                tokens.matchTokenType("WHITESPACE");
                arg = tokenArr
                    .map(function (t) {
                        return t.value;
                    })
                    .join("");
            }

            if (tokens.matchToken("when")) {
                var when = parser.requireElement("expression", tokens);
            }

            var hideShowStrategy = resolveHideShowStrategy(parser, tokens, name);

            return {
                target: targetExpr,
                when: when,
                args: [targetExpr],
            };
        }
    });

    parser.addCommand("take", function (parser, runtime, tokens) {
        if (tokens.matchToken("take")) {
            let classRef = null;
            let classRefs = [];
            while ((classRef = parser.parseElement("classRef", tokens))) {
                classRefs.push(classRef);
            }

            var attributeRef = null;
            var replacementValue = null;

            let weAreTakingClasses = classRefs.length > 0;
            if (!weAreTakingClasses) {
                attributeRef = parser.parseElement("attributeRef", tokens);
                if (attributeRef == null) {
                    parser.raiseParseError(tokens, "Expected either a class reference or attribute expression");
                }

                if (tokens.matchToken("with")) {
                    replacementValue = parser.requireElement("expression", tokens);
                }
            }

            if (tokens.matchToken("from")) {
                var fromExpr = parser.requireElement("expression", tokens);
            }

            if (tokens.matchToken("for")) {
                var forExpr = parser.requireElement("expression", tokens);
            } else {
                var forExpr = parser.requireElement("implicitMeTarget", tokens);
            }

            if (weAreTakingClasses) {
                var takeCmd = {
                    classRefs: classRefs,
                    from: fromExpr,
                    forElt: forExpr,
                    args: [classRefs, fromExpr, forExpr],
                };
                return takeCmd;
            } else {
                //@ts-ignore
                var takeCmd = {
                    attributeRef: attributeRef,
                    from: fromExpr,
                    forElt: forExpr,
                    args: [fromExpr, forExpr, replacementValue],
                };
                return takeCmd;
            }
        }
    });

    function putInto(runtime, context, prop, valueToPut) {
        if (prop != null) {
            var value = runtime.resolveSymbol(prop, context);
        } else {
            var value = context;
        }
        if (value instanceof Element || value instanceof HTMLDocument) {
            while (value.firstChild) value.removeChild(value.firstChild);
            value.append(parser.runtime.convertValue(valueToPut, "Fragment"));
            runtime.processNode(value);
        } else {
            if (prop != null) {
                runtime.setSymbol(prop, context, null, valueToPut);
            } else {
                throw "Don't know how to put a value into " + typeof context;
            }
        }
    }

    parser.addCommand("put", function (parser, runtime, tokens) {
        if (tokens.matchToken("put")) {
            var value = parser.requireElement("expression", tokens);

            var operationToken = tokens.matchAnyToken("into", "before", "after");

            if (operationToken == null && tokens.matchToken("at")) {
                tokens.matchToken("the"); // optional "the"
                operationToken = tokens.matchAnyToken("start", "end");
                tokens.requireToken("of");
            }

            if (operationToken == null) {
                parser.raiseParseError(tokens, "Expected one of 'into', 'before', 'at start of', 'at end of', 'after'");
            }
            var target = parser.requireElement("expression", tokens);

            var operation = operationToken.value;

            var arrayIndex = false;
            var symbolWrite = false;
            var rootExpr = null;
            var prop = null;

            if (target.type === "arrayIndex" && operation === "into") {
                arrayIndex = true;
                prop = target.prop;
                rootExpr = target.root;
            } else if (target.prop && target.root && operation === "into") {
                prop = target.prop.value;
                rootExpr = target.root;
            } else if (target.type === "symbol" && operation === "into") {
                symbolWrite = true;
                prop = target.name;
            } else if (target.type === "attributeRef" && operation === "into") {
                var attributeWrite = true;
                prop = target.name;
                rootExpr = parser.requireElement("implicitMeTarget", tokens);
            } else if (target.type === "styleRef" && operation === "into") {
                var styleWrite = true;
                prop = target.name;
                rootExpr = parser.requireElement("implicitMeTarget", tokens);
            } else if (target.attribute && operation === "into") {
                var attributeWrite = target.attribute.type === "attributeRef";
                var styleWrite = target.attribute.type === "styleRef";
                prop = target.attribute.name;
                rootExpr = target.root;
            } else {
                rootExpr = target;
            }

            var putCmd = {
                target: target,
                operation: operation,
                symbolWrite: symbolWrite,
                value: value,
                args: [rootExpr, prop, value],
            };
            return putCmd;
        }
    });

    function parsePseudopossessiveTarget(parser, runtime, tokens) {
        var targets;
        if (
            tokens.matchToken("the") ||
            tokens.matchToken("element") ||
            tokens.matchToken("elements") ||
            tokens.currentToken().type === "CLASS_REF" ||
            tokens.currentToken().type === "ID_REF" ||
            (tokens.currentToken().op && tokens.currentToken().value === "<")
        ) {
            parser.possessivesDisabled = true;
            try {
                targets = parser.parseElement("expression", tokens);
            } finally {
                delete parser.possessivesDisabled;
            }
            // optional possessive
            if (tokens.matchOpToken("'")) {
                tokens.requireToken("s");
            }
        } else if (tokens.currentToken().type === "IDENTIFIER" && tokens.currentToken().value === "its") {
            var identifier = tokens.matchToken("its");
            targets = {
                type: "pseudopossessiveIts",
                token: identifier,
                name: identifier.value,
                evaluate: function (context) {
                    return runtime.resolveSymbol("it", context);
                },
            };
        } else {
            tokens.matchToken("my") || tokens.matchToken("me"); // consume optional 'my'
            targets = parser.parseElement("implicitMeTarget", tokens);
        }
        return targets;
    }

    parser.addCommand("transition", function (parser, runtime, tokens) {
        if (tokens.matchToken("transition")) {
            var targetsExpr = parsePseudopossessiveTarget(parser, runtime, tokens);

            var properties = [];
            var from = [];
            var to = [];
            var currentToken = tokens.currentToken();
            while (
                !parser.commandBoundary(currentToken) &&
                currentToken.value !== "over" &&
                currentToken.value !== "using"
            ) {
                if (tokens.currentToken().type === "STYLE_REF") {
                    let styleRef = tokens.consumeToken();
                    let styleProp = styleRef.value.substr(1);
                    properties.push({
                        type: "styleRefValue",
                        evaluate: function () {
                            return styleProp;
                        },
                    });
                } else {
                    properties.push(parser.requireElement("stringLike", tokens));
                }

                if (tokens.matchToken("from")) {
                    from.push(parser.requireElement("expression", tokens));
                } else {
                    from.push(null);
                }
                tokens.requireToken("to");
                if (tokens.matchToken("initial")) {
                    to.push({
                        type: "initial_literal",
                        evaluate: function () {
                            return "initial";
                        }
                    });
                } else {
                    to.push(parser.requireElement("expression", tokens));
                }
                currentToken = tokens.currentToken();
            }
            if (tokens.matchToken("over")) {
                var over = parser.requireElement("expression", tokens);
            } else if (tokens.matchToken("using")) {
                var using = parser.requireElement("expression", tokens);
            }

            var transition = {
                to: to,
                args: [targetsExpr, properties, from, to, using, over],
            };
            return transition;
        }
    });

    parser.addCommand("measure", function (parser, runtime, tokens) {
        if (!tokens.matchToken("measure")) return;

        var targetExpr = parsePseudopossessiveTarget(parser, runtime, tokens);

        var propsToMeasure = [];
        if (!parser.commandBoundary(tokens.currentToken()))
            do {
                propsToMeasure.push(tokens.matchTokenType("IDENTIFIER").value);
            } while (tokens.matchOpToken(","));

        return {
            properties: propsToMeasure,
            args: [targetExpr],
        };
    });

    parser.addLeafExpression("closestExpr", function (parser, runtime, tokens) {
        if (tokens.matchToken("closest")) {
            if (tokens.matchToken("parent")) {
                var parentSearch = true;
            }

            var css = null;
            if (tokens.currentToken().type === "ATTRIBUTE_REF") {
                var attributeRef = parser.requireElement("attributeRefAccess", tokens, null);
                css = "[" + attributeRef.attribute.name + "]";
            }

            if (css == null) {
                var expr = parser.requireElement("expression", tokens);
                if (expr.css == null) {
                    parser.raiseParseError(tokens, "Expected a CSS expression");
                } else {
                    css = expr.css;
                }
            }

            if (tokens.matchToken("to")) {
                var to = parser.parseElement("expression", tokens);
            } else {
                var to = parser.parseElement("implicitMeTarget", tokens);
            }

            var closestExpr = {
                type: "closestExpr",
                parentSearch: parentSearch,
                expr: expr,
                css: css,
                to: to,
                args: [to],
                evaluate: function (context) {
                    throwOnlyParsingIsSupported()
                },
            };

            if (attributeRef) {
                attributeRef.root = closestExpr;
                attributeRef.args = [closestExpr];
                return attributeRef;
            } else {
                return closestExpr;
            }
        }
    });

    parser.addCommand("go", function (parser, runtime, tokens) {
        if (tokens.matchToken("go")) {
            if (tokens.matchToken("back")) {
                var back = true;
            } else {
                tokens.matchToken("to");
                if (tokens.matchToken("url")) {
                    var target = parser.requireElement("stringLike", tokens);
                    var url = true;
                    if (tokens.matchToken("in")) {
                        tokens.requireToken("new");
                        tokens.requireToken("window");
                        var newWindow = true;
                    }
                } else {
                    tokens.matchToken("the"); // optional the
                    var verticalPosition = tokens.matchAnyToken("top", "middle", "bottom");
                    var horizontalPosition = tokens.matchAnyToken("left", "center", "right");
                    if (verticalPosition || horizontalPosition) {
                        tokens.requireToken("of");
                    }
                    var target = parser.requireElement("unaryExpression", tokens);

                    var plusOrMinus = tokens.matchAnyOpToken("+", "-");
                    if (plusOrMinus) {
                        tokens.pushFollow("px");
                        try {
                            var offset = parser.requireElement("expression", tokens);
                        } finally {
                            tokens.popFollow();
                        }
                    }
                    tokens.matchToken("px"); // optional px

                    var smoothness = tokens.matchAnyToken("smoothly", "instantly");

                    var scrollOptions = {
                        block: "start",
                        inline: "nearest"
                    };

                    if (verticalPosition) {
                        if (verticalPosition.value === "top") {
                            scrollOptions.block = "start";
                        } else if (verticalPosition.value === "bottom") {
                            scrollOptions.block = "end";
                        } else if (verticalPosition.value === "middle") {
                            scrollOptions.block = "center";
                        }
                    }

                    if (horizontalPosition) {
                        if (horizontalPosition.value === "left") {
                            scrollOptions.inline = "start";
                        } else if (horizontalPosition.value === "center") {
                            scrollOptions.inline = "center";
                        } else if (horizontalPosition.value === "right") {
                            scrollOptions.inline = "end";
                        }
                    }

                    if (smoothness) {
                        if (smoothness.value === "smoothly") {
                            scrollOptions.behavior = "smooth";
                        } else if (smoothness.value === "instantly") {
                            scrollOptions.behavior = "instant";
                        }
                    }
                }
            }

            var goCmd = {
                target: target,
                args: [target, offset],
            };
            return goCmd;
        }
    });
}

function throwOnlyParsingIsSupported() {
    throw new Error('only parsing is supported')
}

/** @param {Token} token */
function jsonifyToken(token) {
    return {
        type: token.type ?? "",
        value: token.value,
        start: token.start ?? 0,
        end: token.end ?? 0,
        line: token.line ?? 1,
        column: (token.column ?? 0) + 1 //Make column positions start at 1.
    }
}

const parser = new Parser()
hyperscriptCoreGrammar(parser)
hyperscriptWebGrammar(parser)
