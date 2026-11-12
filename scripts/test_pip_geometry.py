"""Curve-space geometry checks; run with the vector-tool development venv."""

import unittest
from pathlib import Path
import xml.etree.ElementTree as ET
import pathops
from import_creator_avatars import shape, ellipse, op, visible, PIP_EYES

ROOT = Path(__file__).resolve().parents[1] / "web/static/img/pip"


def grow(p, distance):
    border = pathops.Path(p)
    border.stroke(
        2 * distance, pathops.LineCap.ROUND_CAP, pathops.LineJoin.ROUND_JOIN, 4
    )
    border.convertConicsToQuads(0.001)
    return op(p, border, pathops.PathOp.UNION)


def union(paths):
    result = pathops.Path()
    for p in paths:
        result = op(result, p, pathops.PathOp.UNION)
    return result


class PipGeometry(unittest.TestCase):
    def test_eye_anchored_cheeks_are_symmetric_ovals_with_eight_to_fourteen_unit_gap(
        self,
    ):
        base = ET.parse(ROOT / "base.svg").getroot()
        body = next(g for g in base if g.get("data-pip-part") == "body")
        painted = visible([("part", el.get("fill"), shape(el.get("d"))) for el in body])
        face = union(p for _, fill, p in painted if fill == "#fff6f3")
        for key in ["neutral", "happy", "surprised", "skeptical", "angry", "sad"]:
            with self.subTest(expression=key):
                group = ET.parse(ROOT / ("face-" + key + ".svg")).getroot()[0]
                cheeks = next(g for g in group if g.get("data-pip-part") == "cheeks")
                self.assertEqual(len(cheeks), 2)
                eyes = union(
                    shape(el.get("d"))
                    for el in group
                    if el.tag.endswith("path")
                    and el.get("fill") in ["#080808", "#14141f"]
                )
                safety = grow(eyes, 8)
                vertical_gaps = []
                sizes = []
                for index, el in enumerate(cheeks):
                    p = shape(el.get("d"))
                    x1, y1, x2, y2 = p.bounds
                    eye = union(
                        shape(el.get("d"))
                        for el in group
                        if el.tag.endswith("path")
                        and el.get("fill") in ["#080808", "#14141f"]
                        and (
                            (
                                shape(el.get("d")).bounds[0]
                                + shape(el.get("d")).bounds[2]
                            )
                            / 2
                            < 211
                        )
                        == (index == 0)
                    )
                    vertical_gaps.append(y1 - eye.bounds[3])
                    sizes.append((x2 - x1, y2 - y1))
                    eye_cx, _, eye_rx, _ = PIP_EYES[index]
                    width = eye_rx * 2 * (1.1 if key == "surprised" else 1)
                    outward = (
                        ((x1 + x2) / 2 - eye_cx) * (-1 if index == 0 else 1) / width
                    )
                    self.assertGreaterEqual(outward, 0.2 - 0.001)
                    self.assertLessEqual(outward, 0.3 + 0.001)
                    self.assertGreater(
                        op(p, grow(eye, 14), pathops.PathOp.INTERSECTION).area,
                        0.001,
                        "eye gap above 14",
                    )
                    self.assertGreaterEqual(vertical_gaps[-1], 8 - 0.001)
                    self.assertLessEqual(vertical_gaps[-1], 12 + 0.001)
                    oval = ellipse(
                        (x1 + x2) / 2, (y1 + y2) / 2, (x2 - x1) / 2, (y2 - y1) / 2
                    )
                    self.assertLess(
                        op(p, oval, pathops.PathOp.XOR).area,
                        0.01,
                        "clipped/non-oval cheek",
                    )
                    self.assertLess(
                        op(p, face, pathops.PathOp.DIFFERENCE).area,
                        0.001,
                        "cheek outside white face",
                    )
                    self.assertLess(
                        op(p, safety, pathops.PathOp.INTERSECTION).area,
                        0.001,
                        "eye gap below 8",
                    )

                self.assertAlmostEqual(vertical_gaps[0], vertical_gaps[1], delta=0.002)
                self.assertAlmostEqual(sizes[0][0], sizes[1][0], delta=0.002)
                self.assertAlmostEqual(sizes[0][1], sizes[1][1], delta=0.002)


if __name__ == "__main__":
    unittest.main()
