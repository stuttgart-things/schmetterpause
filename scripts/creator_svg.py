"""Flatten trusted Creator SVG path coordinates for the avatar importer."""

from pathlib import Path
import xml.etree.ElementTree as ET, re
from fontTools.svgLib.path import parse_path
from fontTools.pens.transformPen import TransformPen
from fontTools.misc.transform import Transform
import pathops


def transform(s):
    t = Transform()
    for op, args in re.findall(r"([a-zA-Z]+)\(([^)]*)\)", s or ""):
        v = list(map(float, re.findall(r"[-+]?(?:\d*\.\d+|\d+)", args)))
        if op == "translate":
            t = t.translate(v[0], v[1] if len(v) > 1 else 0)
        elif op == "scale":
            t = t.scale(v[0], v[1] if len(v) > 1 else v[0])
        elif op == "matrix":
            t = t.transform(v)
        else:
            raise ValueError(op)
    return t


def paths(source, name):
    root = ET.parse(Path(source) / (name + ".svg")).getroot()
    w, h = list(map(float, root.get("viewBox").split()))[2:]
    scale = min(420 / w, 420 / h)
    norm = Transform(scale, 0, 0, scale, (420 - w * scale) / 2, 420 - h * scale)
    found = []

    def walk(el, t, role, fill):
        t = t.transform(transform(el.get("transform")))
        role = el.get("data-part", role)
        fill = el.get("fill", fill)
        if el.tag.endswith("path"):
            p = pathops.Path()
            parse_path(el.get("d"), TransformPen(p.getPen(), t))
            p.fillType = (
                pathops.FillType.EVEN_ODD
                if el.get("fill-rule") == "evenodd"
                else pathops.FillType.WINDING
            )
            found.append((role, fill, p))
        else:
            for child in el:
                walk(child, t, role, fill)

    walk(root, norm, None, None)
    return found
