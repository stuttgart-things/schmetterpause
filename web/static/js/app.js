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

// The steps under the score boxes. There is no HTML that puts a button in
// charge of another input, so this is the second thing HTMX does not reach.
//
// One delegated listener rather than one per button: the set rows are swapped
// out whenever the mode or a player changes, and listeners bound to the old
// elements would go with them.
document.addEventListener('click', function (event) {
	var button = event.target.closest && event.target.closest('.step');
	if (!button) {
		return;
	}

	var box = document.getElementById(button.dataset.score);
	if (!box) {
		return;
	}

	var next = (parseInt(box.value, 10) || 0) + parseInt(button.dataset.step, 10);
	// Down from zero lands on the target score instead of stopping there. A
	// set has a winner and the winner has eleven, so one press says the most
	// common thing there is to say about a side — and the presses after it
	// count down through the scores the other side gets.
	if (next < 0) {
		next = parseInt(button.dataset.target, 10) || 0;
	}
	// No ceiling, for the reason the box has no max: 12:10 and 13:11 are
	// ordinary results, and a cap that allowed those would allow anything
	// worth capping anyway.
	box.value = String(next);
	mark(box.closest('.set'));
});

// Typing replaces rather than appends. Every box comes up with a zero in it,
// and on a phone that zero is behind the cursor when the keypad opens — so
// tapping a box and typing eleven produced 011 until this line existed.
document.addEventListener('focusin', function (event) {
	var el = event.target;
	if (el.type === 'number' && el.closest && el.closest('.set')) {
		el.select();
	}
});

// Whatever was typed, the row it was typed into gets marked. See below for
// what the mark is for.
document.addEventListener('input', function (event) {
	var el = event.target;
	mark(el.closest && el.closest('.set'));
});

// A row that still stands at 0:0 was not played, so it keeps its digits quiet.
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
// Delegated, like the steps above: the form is swapped in and out of the
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
