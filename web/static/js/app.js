// The only handwritten JavaScript in the application. Invariant 7 allows it
// where HTMX does not reach, and this is that place.
//
// A rejected form comes back as 422 with the form re-rendered and the reason
// inside it. HTMX swaps 2xx responses only, so without this the response is
// dropped on the floor and the page does nothing at all — which is the worst
// possible answer to a wrong score, because it looks like the button is
// broken rather than like the result is.
//
// The alternative was to answer 200 for a rejected form. The status is worth
// keeping honest: it is what the tests assert on and what a log is read with.
document.addEventListener('htmx:beforeSwap', function (event) {
	if (event.detail.xhr.status === 422) {
		event.detail.shouldSwap = true;
		event.detail.isError = false;
	}
});
