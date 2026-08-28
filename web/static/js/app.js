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
//
// 429 is here for the same reason. The sign-in form answers it when the brake
// on guessing is holding an attempt back, and the response carries the one
// thing somebody needs at that moment: how long is left. Dropping it would
// leave them pressing a button that does nothing.
document.addEventListener('htmx:beforeSwap', function (event) {
	var status = event.detail.xhr.status;
	if (status === 422 || status === 429) {
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

// The reveal button on the sign-in field. There is no HTML that turns a
// password field into a text field, so this is the third thing HTMX does not
// reach.
//
// It exists because of what the field holds: sixteen characters read off a
// password manager and typed into a phone. Typed blind, a single wrong
// character comes back as "das passt nicht", which is indistinguishable from
// having the wrong code entirely — and that is the dead end this whole way
// back was built to remove.
//
// Delegated, like the sliders above: the form is swapped in and out of the
// page by HTMX, and a listener bound to the button would go with it.
document.addEventListener('click', function (event) {
	var button = event.target.closest && event.target.closest('.secret-reveal');
	if (!button) {
		return;
	}

	var field = document.getElementById(button.dataset.reveal);
	if (!field) {
		return;
	}

	var shown = field.type === 'text';
	field.type = shown ? 'password' : 'text';
	button.textContent = shown ? 'Zeigen' : 'Verbergen';
	button.setAttribute('aria-pressed', shown ? 'false' : 'true');
	// Back to where they were typing, rather than leaving focus on a button
	// they have to tab off again.
	field.focus();
});
