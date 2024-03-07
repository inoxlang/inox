package htmx

var (
	EXTENSIONS = map[string]Extension{
		"debug": {
			ShortDescription: "https://htmx.org/extensions/debug",
			Documentation: "This extension logs all htmx events for the element it is on, either through the console.debug" +
				"function or through the console.log function with a DEBUG: prefix. https://htmx.org/extensions/debug",
		},
		"event-header": {
			ShortDescription: "https://htmx.org/extensions/event-header",
			Documentation: "This extension adds the Triggering-Event header to requests. " +
				"The value of the header is a JSON serialized version of the event that triggered the request." +
				" https://htmx.org/extensions/event-header",
		},
		"loading-states": {
			ShortDescription: "https://htmx.org/extensions/loading-states",
			Documentation: "This extension allows you to easily manage loading states while a request is in flight," +
				"including disabling elements, and adding and removing CSS classes. https://htmx.org/extensions/loading-states",
		},
		"morphdom-swap": {
			ShortDescription: "https://htmx.org/extensions/morphdom-swap (comes with morphdom)",
			Documentation: "This extension allows you to use the morphdom library (packaged with the extension) as the swapping mechanism in htmx.\n\n" +
				"The morphdom library does not support morph element to multiple elements. If the result of hx-select " +
				"is more than one element, it will pick the first one. https://htmx.org/extensions/morphdom-swap",
		},
		"multi-swap": {
			ShortDescription: "https://htmx.org/extensions/multi-swap",
			Documentation: "This extension allows you to swap multiple elements marked with the id attribute from the HTML response. " +
				"You can also choose for each element which swap method should be used.\n\n" +

				"Multi-swap can help in cases where OOB (Out of Band Swaps) is not enough for you. OOB requires HTML tags marked with " +
				"hx-swap-oob attributes to be at the TOP level of HTML, which significantly limited its use. With OOB, it’s impossible " +
				"to swap multiple elements arbitrarily placed and nested in the DOM tree.\n\n" +

				"It is a very powerful tool in conjunction with hx-boost and preload extension. https://htmx.org/extensions/multi-swap",
		},
		"path-deps": {
			ShortDescription: "https://htmx.org/extensions/path-deps",
			Documentation:    "This extension supports expressing inter-element dependencies based on paths. https://htmx.org/extensions/path-deps",
		},
		"response-targets": {
			ShortDescription: "https://htmx.org/extensions/response-targets",
			Documentation: "This extension allows you to specify different target elements to be swapped when different HTTP response codes " +
				"are received.\n\n" +

				"It uses attribute names in a form of hx-target-[CODE] where [CODE] is a numeric HTTP response code with the optional wildcard " +
				"character at its end. You can also use hx-target-error, which handles both 4xx and 5xx response codes." +
				" https://htmx.org/extensions/response-targets,",
		},
		"restored": {
			ShortDescription: "https://htmx.org/extensions/restored",
			Documentation: "This extension triggers an event restored whenever a back button even is detected while using hx-boost." +
				" https://htmx.org/extensions/restored",
		},
		"sse": {
			ShortDescription: "https://htmx.org/extensions/server-sent-events",
			Documentation: "The Server Sent Events extension connects to an EventSource directly from HTML. It manages the connections to your web server, " +
				"listens for server events, and then swaps their contents into your htmx webpage in real-time." +
				" https://htmx.org/extensions/server-sent-events",
		},
		"ws": {
			ShortDescription: "https://htmx.org/extensions/web-sockets",
			Documentation: "The WebSockets extension enables easy, bi-directional communication with Web Sockets servers directly from HTML." +
				" https://htmx.org/extensions/web-sockets",
		},
		"json-form": {
			ShortDescription: "required for sending JSON with forms",
		},
	}
)

type Extension struct {
	Name             string
	Code             string
	MinifiedCode     string
	ShortDescription string
	Documentation    string
}
