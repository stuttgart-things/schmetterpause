#!/usr/bin/env python3
"""Historical B-variant entry point; regenerate the current mitten review."""
from pathlib import Path
import sys
sys.path.insert(0,str(Path(__file__).resolve().parents[4]/'scripts'))
from hank_completion_review import regenerate
if __name__=='__main__':regenerate()
