//https://github.com/bigskysoftware/htmx/blob/f919c0705182c904a440e3ff4a9687f4d5166c55/dist/ext/debug.js
htmx.defineExtension('debug', {
    onEvent: function (name, evt) {
        if (console.debug) {
            console.debug(name, evt);
        } else if (console) {
            console.log("DEBUG:", name, evt);
        } else {
            throw "NO CONSOLE SUPPORTED"
        }
    }
});