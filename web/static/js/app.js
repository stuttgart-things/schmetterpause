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

// The sliders under the score boxes. There is no HTML that links two inputs,
// so this is the second thing HTMX does not reach.
//
// One delegated listener rather than one per slider: the set rows are swapped
// out whenever the mode or a player changes, and listeners bound to the old
// elements would go with them.
document.addEventListener('input', function (event) {
	var el = event.target;

	if (el.classList && el.classList.contains('score-slider')) {
		var box = document.getElementById(el.dataset.score);
		if (box) {
			box.value = el.value;
		}
		// Marking before returning, not after: an early return here left a
		// row dim after the slider had just put a number in it.
		mark(el.closest && el.closest('.set'));
		return;
	}

	// Typing in the box moves the slider back under it. Without this the two
	// disagree the moment somebody uses the keypad, and the slider then jumps
	// from a stale position the next time it is touched.
	if (el.id) {
		var slider = document.querySelector('.score-slider[data-score="' + el.id + '"]');
		if (slider && el.value !== '') {
			slider.value = el.value;
		}
	}

	mark(el.closest && el.closest('.set'));
});

// A row that still stands at 0:0 was not played, so its two sliders are dim.
// Marked per row rather than per box: 11:0 is a real set, and dimming the
// zero in it would call a result an absence.
function mark(row) {
	if (!row) {
		return;
	}
	var played = false;
	row.querySelectorAll('input[type="number"]').forEach(function (box) {
		if (box.value !== '' && box.value !== '0') {
			played = true;
		}
	});
	row.classList.toggle('has-score', played);
}

function markAll() {
	document.querySelectorAll('.set').forEach(mark);
}

// The set rows are replaced whenever the mode or a player changes, and the
// replacement arrives with whatever was already typed in it.
document.addEventListener('htmx:afterSwap', markAll);
document.addEventListener('DOMContentLoaded', markAll);
