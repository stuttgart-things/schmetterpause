#!/usr/bin/env python3
"""One-time OFL Barlow Bold outline import; never used by the application."""

import argparse
import hashlib
import json
from pathlib import Path
from fontTools.ttLib import TTFont
from fontTools.pens.recordingPen import DecomposingRecordingPen
from fontTools.pens.svgPathPen import SVGPathPen

parser = argparse.ArgumentParser()
parser.add_argument("font", type=Path)
parser.add_argument(
    "--out", type=Path, default=Path(__file__).with_name("hank_glyphs.json")
)
args = parser.parse_args()
font = TTFont(args.font)
glyphs = font.getGlyphSet()
result = {
    "font": "Barlow Bold",
    "license": "SIL Open Font License 1.1",
    "source": "https://github.com/google/fonts/tree/main/ofl/barlow",
    "font_sha256": hashlib.sha256(args.font.read_bytes()).hexdigest(),
    "glyphs": {},
}
for letter in "ABCDEFGHIJKLMNOPQRSTUVWXYZÄÖÜ":
    name = font.getBestCmap()[ord(letter)]
    recording = DecomposingRecordingPen(glyphs)
    glyphs[name].draw(recording)
    pen = SVGPathPen(None)
    recording.replay(pen)
    result["glyphs"][letter] = pen.getCommands()
args.out.write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n")
