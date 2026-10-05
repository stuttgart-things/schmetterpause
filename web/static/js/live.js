// The running score on the start page (#188): the Zählwerk's table stream,
// written into the #live section that internal/templates/live.templ renders.
//
// Handwritten rather than HTMX, and that is invariant 7's exception rather
// than a way around it: the stream carries JSON by design (zaehlwerk#4 says
// why — its clients render differently), and HTMX's SSE extension can only
// swap HTML. It stays this small on purpose: no build step, nothing it does is
// needed for the page to work, and it computes nothing. The names come from
// the stream, the ratings from the page, and the score is written out as it
// arrives.
(function () {
	var section = document.getElementById('live');
	if (!section || !window.EventSource) {
		return;
	}

	// player_id -> stored TTR, rendered by the server. A match nobody reports
	// carries no ids, and then the names stand alone.
	var ratings = {};
	try {
		ratings = JSON.parse(document.getElementById('live-ratings').textContent) || {};
	} catch (e) {
		ratings = {};
	}

	var sides = section.querySelectorAll('.live-side');
	var setLine = section.querySelector('.live-set');
	var source = new EventSource(section.dataset.stream);

	source.onmessage = function (message) {
		var event;
		try {
			event = JSON.parse(message.data);
		} catch (e) {
			return;
		}

		// Between matches, and once one is over. The stream follows the table
		// by itself, so the next match's snapshot brings the section back
		// without anything here having to ask.
		var state = event.state;
		if (event.kind === 'no_match' || !state || state.complete) {
			section.hidden = true;
			return;
		}

		var ids = [event.home_id, event.away_id];
		for (var i = 0; i < sides.length; i++) {
			var side = sides[i];
			var ttr = ids[i] ? ratings[ids[i]] : undefined;
			// textContent throughout: names are typed at the table, and none
			// of this is markup.
			side.querySelector('.live-name').textContent = state.players[i] || '';
			side.querySelector('.live-ttr').textContent = ttr ? 'TTR ' + ttr : '';
			// "Sätze 1" rather than "1 Satz", which reads like the set number
			// printed under the board.
			side.querySelector('.live-sets').textContent = 'Sätze ' + state.sets[i];
			side.querySelector('.live-points').textContent = state.points[i];
			side.classList.toggle('live-serving', state.serving === (i === 0 ? 'a' : 'b'));
		}
		setLine.textContent = 'Satz ' + state.set_number;
		section.hidden = false;
	};

	// A dropped connection leaves a score that is no longer moving, which
	// reads as the match being stuck. Hidden instead; EventSource reconnects
	// by itself and the Zählwerk answers a new connection with the current
	// snapshot.
	source.onerror = function () {
		section.hidden = true;
	};
})();
