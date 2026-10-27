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
//
// Beside each name stands that player's figure (docs/adr/0029), fetched once
// per match from /avatars/{id}.svg, or /avatars/random.svg for a side with
// nobody behind it, such as a demo match. Which face it shows is the
// Zählwerk's call, sent as faces in the event; this only sets data-face, and
// shows the neutral one while the stream carries none.
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
	// Which figure each side shows: a player id, or the match a random one
	// was rolled for. A new key fetches; the same key only changes the face.
	var figureKeys = [];
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
			showFigure(side, i, ids[i], state.match_id, event.faces ? event.faces[i] : '');
		}
		setLine.textContent = 'Satz ' + state.set_number;
		section.hidden = false;
	};

	// showFigure keeps side i's figure in step with the event: fetched when
	// the player or the match changes, its face set on every event.
	function showFigure(side, i, id, matchID, face) {
		var box = side.querySelector('.live-figure');
		var key = id ? 'player:' + id : 'match:' + matchID;
		if (figureKeys[i] !== key) {
			figureKeys[i] = key;
			box.textContent = '';
			var url = id ? '/avatars/' + encodeURIComponent(id) + '.svg' : '/avatars/random.svg';
			fetch(url).then(function (response) {
				return response.ok ? response.text() : '';
			}).then(function (svg) {
				// A later event may have moved the side on while this was
				// loading; its own fetch will fill the box.
				if (figureKeys[i] !== key) {
					return;
				}
				// innerHTML for this one string only: it is this
				// application's own figure, assembled by avatar.Render from
				// parts in the binary and colours from its palettes, served
				// from the same origin. Nothing typed at the table is in it.
				box.innerHTML = svg;
				setFace(box, side.dataset.face);
				section.classList.toggle('live-figures', svg !== '');
			}).catch(function () {
				// No figure is not worth a word: the score is the point.
			});
		}
		side.dataset.face = face || 'neutral';
		setFace(box, side.dataset.face);
	}

	function setFace(box, face) {
		var svg = box.querySelector('svg');
		if (svg) {
			svg.setAttribute('data-face', face);
		}
	}

	// A dropped connection leaves a score that is no longer moving, which
	// reads as the match being stuck. Hidden instead; EventSource reconnects
	// by itself and the Zählwerk answers a new connection with the current
	// snapshot.
	source.onerror = function () {
		section.hidden = true;
	};
})();
