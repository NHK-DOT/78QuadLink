#!/usr/bin/env python3
"""Render the checked-in SVG to PNG/ICO using Qt's SVG rasterizer and Pillow."""
from pathlib import Path
import os
os.environ.setdefault('QT_QPA_PLATFORM', 'offscreen')
from PyQt5.QtWidgets import QApplication
from PyQt5.QtSvg import QSvgRenderer
from PyQt5.QtGui import QImage, QPainter
from PIL import Image
root = Path(__file__).resolve().parents[1]
app = QApplication([])
canvas = QImage(512, 512, QImage.Format_ARGB32)
canvas.fill(0)
painter = QPainter(canvas)
QSvgRenderer(str(root/'assets/quadlink78.svg')).render(painter)
painter.end()
canvas.save(str(root/'assets/quadlink78.png'))
Image.open(root/'assets/quadlink78.png').save(root/'assets/quadlink78.ico', sizes=[(16,16),(32,32),(48,48),(64,64),(128,128),(256,256)])
