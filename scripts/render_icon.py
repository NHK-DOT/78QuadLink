#!/usr/bin/env python3
"""Render a transparent pixel ICO and a U/lightning + 78Link PNG wordmark."""
from pathlib import Path
from PIL import Image

# U silhouette reference: https://www.unitree.com/unitree-favicon.svg
# Pixel adaptation + lightning, 2026-09-19. '#' black, '.' white; no smoothing.
PIXELS = """
................................
................................
................................
................................
................................
.....#########.........########.
.....#########........#########.
.....########.........#########.
.....########.....##..########..
....#########....###..########..
....#########....##..#########..
....########....###..#########..
....########...###...########...
...#########...#####.########...
...#########..###############...
...########.....###.#########...
...########.....##..########....
..#########....###..########....
..#########....##..#########....
..########....##...#########....
..########....#....########.....
.#########.........########.....
.#########........#########.....
.##########......##########.....
.#########################......
.#########################......
..######################........
................................
................................
................................
................................
................................
""".strip().splitlines()
root = Path(__file__).resolve().parents[1]
base = Image.new('RGBA', (32, 32), (0,0,0,0))
assert len(PIXELS) == 32 and all(len(row) == 32 for row in PIXELS)
for y, row in enumerate(PIXELS):
    for x, pixel in enumerate(row):
        if pixel == '#':
            base.putpixel((x,y), (0,0,0,255))
# Separate the bolt from the original silhouette, then shift it one pixel left.
bolt = [(8,18,19),(9,17,19),(10,17,18),(11,16,18),(12,15,17),
        (13,15,19),(14,14,19),(15,16,18),(16,16,17),(17,15,17),
        (18,15,16),(19,14,15),(20,14,14)]
for y, start, end in bolt:
    for x in range(start, end+1):
        base.putpixel((x,y), (0,0,0,0))
for y, start, end in bolt:
    for x in range(start-1, end):
        base.putpixel((x,y), (0,0,0,255))
nearest = getattr(Image, 'Resampling', Image).NEAREST
frames = [base.resize((n,n), nearest) for n in (16,32,48)]
frames[-1].save(root/'assets/quadlink78.ico', format='ICO', sizes=[(16,16),(32,32),(48,48)],
               append_images=frames[:-1], bitmap_format='bmp')

# Original lettering inspired by Unitree's bold, forward-slanted geometric
# wordmark (not an extracted or redistributed proprietary font).
# Keep the requested "78Link" spelling and a hard-edged pixel grid.
glyphs = {
 '7': ['#########','#########','#########','......###','.....###.','....###..','...###...','..###....','.###.....','###......','###......'],
 '8': ['.#######.','#########','###...###','###...###','.#######.','.#######.','###...###','###...###','###...###','#########','.#######.'],
 'L': ['###......']*8 + ['#########']*3,
 'i': ['###','###','...','...'] + ['###']*7,
 'n': ['.........']*3 + ['########.','#########'] + ['###...###']*6,
 'k': ['###......']*3 + ['###...###','###..###.','###.###..','######...','#######..','###.###..','###..###.','###...###'],
}
text_width = sum(len(glyphs[c][0])+2 for c in '78Link')-2
upright = Image.new('RGBA', (text_width,11), (0,0,0,0))
x = 0
for c in '78Link':
    for y, row in enumerate(glyphs[c]):
        for dx, pixel in enumerate(row):
            if pixel == '#':
                upright.putpixel((x+dx,y), (0,0,0,255))
    x += len(glyphs[c][0])+2
# About 16 degrees of forward lean, matching the direction of the U.
text = Image.new('RGBA', (upright.width+3,upright.height),(0,0,0,0))
for y in range(upright.height):
    shift = round((upright.height-1-y)*.28)
    for x in range(upright.width):
        text.putpixel((x+shift,y),upright.getpixel((x,y)))
text = text.resize((text.width*2,text.height*2), nearest)
symbol = base.crop(base.getbbox())
logo = Image.new('RGBA', (symbol.width+6+text.width,symbol.height),(0,0,0,0))
logo.alpha_composite(symbol,(0,0))
logo.alpha_composite(text,(symbol.width+6,(symbol.height-text.height)//2))
logo = logo.crop(logo.getbbox())
logo.resize((logo.width*6,logo.height*6),nearest).save(root/'assets/quadlink78.png')
