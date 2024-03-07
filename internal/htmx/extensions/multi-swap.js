//https://github.com/bigskysoftware/htmx/blob/ba2c30b648d64be50a127ca4767eae41a4bc266f/src/ext/multi-swap.js
(function () {

    /** @type {import("../htmx").HtmxInternalApi} */
    var api;

    htmx.defineExtension('multi-swap', {
        init: function (apiRef) {
            api = apiRef;
        },
        isInlineSwap: function (swapStyle) {
            return swapStyle.indexOf('multi:') === 0;
        },
        handleSwap: function (swapStyle, target, fragment, settleInfo) {
            if (swapStyle.indexOf('multi:') === 0) {
                var selectorToSwapStyle = {};
                var elements = swapStyle.replace(/^multi\s*:\s*/, '').split(/\s*,\s*/);

                elements.map(function (element) {
                    var split = element.split(/\s*:\s*/);
                    var elementSelector = split[0];
                    var elementSwapStyle = typeof (split[1]) !== "undefined" ? split[1] : "innerHTML";

                    if (elementSelector.charAt(0) !== '#') {
                        console.error("HTMX multi-swap: unsupported selector '" + elementSelector + "'. Only ID selectors starting with '#' are supported.");
                        return;
                    }

                    selectorToSwapStyle[elementSelector] = elementSwapStyle;
                });

                for (var selector in selectorToSwapStyle) {
                    var swapStyle = selectorToSwapStyle[selector];
                    var elementToSwap = fragment.querySelector(selector);
                    if (elementToSwap) {
                        api.oobSwap(swapStyle, elementToSwap, settleInfo);
                    } else {
                        console.warn("HTMX multi-swap: selector '" + selector + "' not found in source content.");
                    }
                }

                return true;
            }
        }
    });
})();