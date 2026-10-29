"""Render a truecolor ANSI frame to a terminal-accurate PNG screenshot.

Cells use the font's real line height so box-drawing characters connect, wide
characters (emoji) take two cells, and emoji are drawn with Noto Color Emoji.
Usage: shot.py in.ansi out.png BG_HEX FG_HEX
"""
import glob
import os
import re
import sys
import unicodedata

from PIL import Image, ImageDraw, ImageFont

SIZE = 16
FONT_DIR = os.path.expanduser("~/.local/share/fonts/JetBrainsMono")
reg = ImageFont.truetype(f"{FONT_DIR}/JetBrainsMonoNerdFontMono-Regular.ttf", SIZE)
bold = ImageFont.truetype(f"{FONT_DIR}/JetBrainsMonoNerdFontMono-Bold.ttf", SIZE)
emoji_font = ImageFont.truetype("/usr/share/fonts/truetype/noto/NotoColorEmoji.ttf", 109)
ascent, descent = reg.getmetrics()
CW = round(reg.getlength("M"))
CH = ascent + descent
PAD = 16

SGR = re.compile(r"\x1b\[([0-9;:]*)m")
OTHER = re.compile(r"\x1b\][^\x07\x1b]*(\x07|\x1b\\)|\x1b\[[0-9;?]*[A-Za-z]")


def hexrgb(h):
    h = h.lstrip("#")
    return tuple(int(h[i:i + 2], 16) for i in (0, 2, 4))


def wide(ch):
    return unicodedata.east_asian_width(ch) in ("W", "F") or ord(ch) >= 0x1F300


def parse(text):
    rows = []
    for line in text.split("\n"):
        cells, fg, bg, b, strike = [], None, None, False, False
        i = 0
        while i < len(line):
            m = SGR.match(line, i)
            if m:
                ps = re.split("[;:]", m.group(1)) if m.group(1) else ["0"]
                j = 0
                while j < len(ps):
                    p = ps[j] or "0"
                    if p == "0":
                        fg, bg, b, strike = None, None, False, False
                    elif p == "1":
                        b = True
                    elif p == "22":
                        b = False
                    elif p == "9":
                        strike = True
                    elif p == "29":
                        strike = False
                    elif p == "39":
                        fg = None
                    elif p == "49":
                        bg = None
                    elif p in ("38", "48") and j + 1 < len(ps) and ps[j + 1] == "2":
                        col = tuple(int(x) for x in ps[j + 2:j + 5])
                        if p == "38":
                            fg = col
                        else:
                            bg = col
                        j += 4
                    j += 1
                i = m.end()
                continue
            m = OTHER.match(line, i)
            if m:
                i = m.end()
                continue
            ch = line[i]
            if unicodedata.category(ch) in ("Mn", "Cf") or ch == "️":
                i += 1
                continue
            cells.append((ch, fg, bg, b, strike))
            if wide(ch):
                cells.append(None)  # continuation cell
            i += 1
        rows.append(cells)
    return rows


def render(src, dst, bg_hex, fg_hex):
    default_bg, default_fg = hexrgb(bg_hex), hexrgb(fg_hex)
    rows = parse(open(src, encoding="utf-8").read())
    cols = max(len(r) for r in rows)
    img = Image.new("RGB", (cols * CW + 2 * PAD, len(rows) * CH + 2 * PAD), default_bg)
    d = ImageDraw.Draw(img)
    for y, row in enumerate(rows):
        for x, cell in enumerate(row):
            if cell is None:
                continue
            ch, fg, bg, b, strike = cell
            px, py = PAD + x * CW, PAD + y * CH
            w = 2 * CW if wide(ch) else CW
            if bg:
                d.rectangle([px, py, px + w - 1, py + CH - 1], fill=bg)
            if ch == " ":
                continue
            if wide(ch) and ord(ch) >= 0x1F300:
                g = Image.new("RGBA", (136, 128), (0, 0, 0, 0))
                ImageDraw.Draw(g).text((0, 0), ch, font=emoji_font, embedded_color=True)
                g = g.resize((w - 2, w - 2), Image.LANCZOS)
                img.paste(g, (px + 1, py + (CH - g.height) // 2), g)
                continue
            color = fg or default_fg
            # Draw block/line glyphs as exact rectangles: font glyphs leave
            # hairline seams between rows that a real terminal doesn't show.
            eighths = "▁▂▃▄▅▆▇█"
            if ch in eighths:
                h = round(CH * (eighths.index(ch) + 1) / 8)
                d.rectangle([px, py + CH - h, px + w - 1, py + CH - 1], fill=color)
                continue
            if ch == "▀":
                d.rectangle([px, py, px + w - 1, py + CH // 2 - 1], fill=color)
                continue
            if ch == "│":
                d.rectangle([px + w // 2, py, px + w // 2, py + CH - 1], fill=color)
                continue
            if ch in "─━":
                t = 1 if ch == "─" else 3
                d.rectangle([px, py + CH // 2 - t // 2, px + w - 1, py + CH // 2 + (t - 1) // 2], fill=color)
                continue
            d.text((px, py), ch, font=bold if b else reg, fill=color)
            if strike:
                d.line([px, py + CH // 2, px + w, py + CH // 2], fill=fg or default_fg)
    img.save(dst, optimize=True)


if __name__ == "__main__":
    render(sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4])
