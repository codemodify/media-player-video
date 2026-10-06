#!/usr/bin/env python3
"""Convert preserved skin artwork into uitoolkit packs. Requires Pillow.

The Zoom importer reads drawing data and arithmetic, never executes SKN code.
Archives and extracted originals live under internal/skins/sources.
"""
import ast
import json
import re
import xml.etree.ElementTree as ET
from pathlib import Path
from PIL import Image, ImageDraw

ROOT = Path(__file__).resolve().parents[1]
SOURCES = ROOT / 'internal/skins/sources'
PACKS = ROOT / 'internal/skins/packs'


def keyed(image, colors=((255, 0, 0), (255, 0, 255))):
    image = image.convert('RGBA')
    image.putdata([(0, 0, 0, 0) if p[:3] in colors else p for p in image.getdata()])
    return image


class Pack:
    def __init__(self, name):
        self.name = name
        self.images = {}
        self.doc = {'skin': 1, 'label': name, 'base': 'win95',
                    'window': {'layout': ':'}, 'sheets': {}, 'sprites': {}, 'layouts': {}}

    def sprite(self, name, image, slices=None):
        self.images[name] = image.convert('RGBA')
        self.doc['sprites'][name] = {'sheet': 'art', 'at': []}
        if slices:
            self.doc['sprites'][name]['slice'] = slices

    def layout(self, name, size, face):
        self.doc['layouts'][name] = {'size': size, 'art': face, 'slots': {}}
        return self.doc['layouts'][name]['slots']

    def write(self):
        width = max(1024, max(im.width for im in self.images.values()))
        x = y = row = 0
        positions = {}
        for name, im in self.images.items():
            if x + im.width > width:
                x = 0; y += row + 2; row = 0
            positions[name] = (x, y)
            self.doc['sprites'][name]['at'] = [x, y, im.width, im.height]
            x += im.width + 2; row = max(row, im.height)
        atlas = Image.new('RGBA', (width, y + row))
        for name, im in self.images.items():
            atlas.paste(im, positions[name])
        path = PACKS / self.name
        (path / 'art').mkdir(parents=True, exist_ok=True)
        atlas.save(path / 'art/skin.png')
        self.doc['sheets']['art'] = {'1x': 'art/skin.png', 'pixelated': False}
        (path / 'skin.json').write_text(json.dumps(self.doc, indent=2) + '\n')
        print('generated', self.name)


def number(expr, variables):
    expr = re.sub(r'<([^>]+)>', lambda m: str(variables[m[1].lower()]), expr).strip()
    def walk(node):
        if isinstance(node, ast.Constant) and isinstance(node.value, (int, float)):
            return node.value
        if isinstance(node, ast.UnaryOp) and isinstance(node.op, ast.USub):
            return -walk(node.operand)
        if isinstance(node, ast.BinOp):
            a, b = walk(node.left), walk(node.right)
            if isinstance(node.op, ast.Add): return a + b
            if isinstance(node.op, ast.Sub): return a - b
            if isinstance(node.op, ast.Mult): return a * b
            if isinstance(node.op, ast.Div): return a / b
        raise ValueError('unsupported skin expression: ' + expr)
    return int(walk(ast.parse(expr, mode='eval').body))


def zoom_lines(text, active=None):
    groups = []; maximized = False
    for line in text.splitlines():
        line = line.strip()
        if line.startswith('//') or line.startswith('/Copy'): continue
        if line.lower() == 'ifmaximized': maximized = True; continue
        if line.lower() == 'endmaximized': maximized = False; continue
        m = re.match(r'Start(And)?Group\(([^)]+)\)', line, re.I)
        if m:
            values = [int(v) for v in m[2].split(',')]
            tests = [v in (1, -1024) if active is None else
                     (v in active if v >= 0 else -v not in active) for v in values]
            groups.append(all(tests) if m[1] else any(tests)); continue
        if re.match(r'End(And)?Group', line, re.I):
            groups.pop(); continue
        if all(groups) and not maximized: yield line


def parameters(line):
    return {k.lower(): v.strip() for k, v in re.findall(r'(\w+)\s*=\s*([^,)]*)', line)}


FUNCTIONS = {'fnexit': 'close', 'fnmax': 'maximize', 'fnminimize': 'minimize',
             'fnfullscreen': 'full', 'fnplay': 'play', 'fnstop': 'stop',
             'fnprevchapter': 'prev', 'fnnextchapter': 'next', 'fnopen': 'open',
             'fnrewind': 'rew', 'fnfastforward': 'ffwd', 'fnplaylist': 'playlist',
             'fnequalizer': 'audio', 'fnaudiomode': 'menu', 'fncustombutton1': 'skins',
             'fnrandomplay': 'shuffle', 'fnloopplay': 'repeat', 'fnpladdfiles': 'add',
             'fnpladddir': 'folder', 'fnplremove': 'remove', 'fnplclear': 'clear',
             'fnplsort':'sort','fnplsavelist':'save','fnplloadlist':'load',
             'fnplitemup':'up','fnplitemdown':'down','fnplmax':'maximize',
             'exgrouptoggle':'menu','fnfitsource':'compact','fnarcycle':'aspect'}


def zoom(name, skn, bitmap, width, height, silver=False, quicktime=False):
    source = Image.open(SOURCES / name / 'original' / bitmap).convert('RGBA')
    lines = list(zoom_lines((SOURCES / name / 'original' / skn).read_text()))
    pack = Pack(name)
    # Each original role retains its own chassis and anchored control geometry.
    for role, w, h in [('main', width, height), ('playlist', 360 if quicktime else 454 if not silver else 412, 370 if quicktime else 294)]:
        variables = {'winwidth': w, 'winheight': h, 'winhalfwidth': w / 2,
                     'vidwidth': w - (12 if silver else 14), 'vidheight': h - (63 if silver else 102),
                     'plwinwidth': w, 'plwinheight': h, 'winhalfheight':h/2,
                     'plwinhalfwidth':w/2}
        if quicktime: variables.update(vidwidth=w-18,vidheight=h-113)
        image = Image.new('RGBA', (w, h))
        centered = []
        prefix = 'pl' if role == 'playlist' else ''
        for line in lines:
            match = re.match(r'(Copy|Tile|Fill)(' + prefix + r')(StretchedBitmap|Bitmap[HV]?|Rect)\((.*)\)', line, re.I)
            if not match: continue
            args = match[4].split(',')
            operation = match[1].lower()
            if operation == 'fill':
                x, y, rw, rh = [number(v, variables) for v in args[:4]]
                color = args[4].strip().upper().removesuffix('NT')
                if len(color) != 6: continue
                if rw > 0 and rh > 0: ImageDraw.Draw(image).rectangle((x, y, x + rw - 1, y + rh - 1), fill='#' + color)
            else:
                sx, sy, sw, sh, dx, dy = [number(v, variables) for v in args[:6]]
                crop = source.crop((sx, sy, sx + sw, sy + sh))
                if name=='zoom-player' and role=='main' and '<WinHalfWidth>' in line and dy>=h-50:
                    centered.append((match[3].lower(),operation,crop,dx-w//2+150,dy-h+50,number(args[6],variables) if operation=='tile' else 0))
                    continue
                if operation == 'copy':
                    if match[3].lower()=='stretchedbitmap':crop=crop.resize((number(args[6],variables),number(args[7],variables)))
                    image.paste(crop, (dx, dy))
                else:
                    length = number(args[6], variables)
                    horizontal = match[3].lower().endswith('h')
                    step = sw if horizontal else sh
                    for offset in range(0, max(0, length), step):
                        tile = crop.crop((0, 0, min(sw, length - offset) if horizontal else sw,
                                          sh if horizontal else min(sh, length - offset)))
                        image.paste(tile, (dx + offset if horizontal else dx, dy if horizontal else dy + offset))
        slices = ([24,175,89,175] if role=='main' else [77,175,42,175]) if quicktime else [21, 91, 42, 39] if silver else [27, 83, 72, 83]
        colors=((0,255,0),) if quicktime else ((255,0,0),(255,0,255))
        pack.sprite('face.' + role, keyed(image,colors), slices)
        slots = pack.layout('video.' + role, [w, h], 'face.' + role)
        if centered:
            transport=image.crop((w//2-150,h-50,w//2+150,h))
            for mode,operation,crop,dx,dy,length in centered:
                if operation=='copy':transport.paste(crop,(dx,dy))
                else:
                    for offset in range(0,length,crop.width):transport.paste(crop.crop((0,0,min(crop.width,length-offset),crop.height)),(dx+offset,dy))
            pack.sprite('face.transport',keyed(transport,colors))
            pack.doc['layouts']['video.transport']={'size':[300,50],'art':'face.transport','slots':{}}
        for line in lines:
            if quicktime and line.lower().startswith('createbutton('):
                args=line[line.index('(')+1:-1].split(',')
                line='CreateExButton('+','.join(k+'='+v for k,v in zip(['SrcX','SrcY','Width','Height','DestX','DestY','Function','Hint','TransColor'],args))+')'
            if not line.lower().startswith('createexbutton('): continue
            values = parameters(line)
            target = values.get('target', 'main').lower()
            if target != ('playlist' if role == 'playlist' else 'main'): continue
            key = FUNCTIONS.get(values.get('function', '').lower())
            if not key or key in slots or not all(k in values for k in ['srcx', 'srcy', 'width', 'height', 'destx', 'desty']): continue
            # Playlist close toggles its own window.
            if role == 'playlist' and key == 'playlist': key = 'close'
            bw, bh = number(values['width'], variables), number(values['height'], variables)
            art = {}
            for state, xkey, ykey in [('normal','srcx','srcy'), ('hover','hoverx','hovery'), ('pressed','downx','downy'), ('checked','activex','activey'), ('checkedHover','activehoverx','activehovery'), ('checkedPressed','activedownx','activedowny')]:
                if xkey not in values or ykey not in values: continue
                sx, sy = number(values[xkey], variables), number(values[ykey], variables)
                sprite = role + '.' + key + '.' + state
                pack.sprite(sprite, keyed(source.crop((sx, sy, sx + bw, sy + bh)),colors))
                art[state] = sprite
            xexpr, yexpr = values['destx'], values['desty']
            x, y = number(xexpr, variables), number(yexpr, variables)
            slot = {'at': [x, y, bw, bh], 'art': art}
            if 'winwidth>' in xexpr.lower(): slot['at'][0] = w - x - bw; slot['fromRight'] = True
            if 'winheight>' in yexpr.lower(): slot['at'][1] = h - y - bh; slot['fromBottom'] = True
            if 'winhalfwidth>' in xexpr.lower() and role=='main':
                # The app centers this fixed transport strip, as the SKN does.
                slot['at'][0] = x - w // 2 + 150
                transport_height=60 if quicktime else 50
                transport = pack.doc['layouts'].setdefault('video.transport', {'size': [300, transport_height], 'slots': {}})
                slot['at'][1] = y - h + transport_height; slot.pop('fromBottom', None)
                transport['slots'][key] = slot
            else: slots[key] = slot
        if role == 'main':
            slots['screen'] = {'at': [6 if silver else 7, 21 if silver else 29, 6 if silver else 7, 42 if silver else 73], 'stretchX': True, 'stretchY': True}
            slots['title'] = {'at': [9 if silver else 52, 6 if silver else 7, 100 if silver else 150, 14], 'stretchX': True}
            if quicktime:
                slots['screen']={'at':[9,24,9,89],'stretchX':True,'stretchY':True}
                slots['title']={'at':[69,5,21,15],'stretchX':True}
                slots['elapsed']={'at':[10,66,44,10],'fromBottom':True}
                slots['seek']={'at':[58,67,51,12],'stretchX':True,'fromBottom':True}
                slots['volume']={'at':[18,36,54,13],'fromBottom':True}
                pack.sprite('seek.thumb',keyed(source.crop((418,356,430,368)),colors))
                for i in range(14):pack.sprite('volume.'+str(i),keyed(source.crop((0,i*13,54,(i+1)*13)),colors))
            elif silver:
                # The original Silverchrome timeline sits alongside the volume.
                slots['seek'] = {'at': [41, 36, 67, 13], 'stretchX': True, 'fromBottom': True}
                slots['volume'] = {'at': [5, 26, 48, 13], 'fromRight': True, 'fromBottom': True}
                pack.sprite('seek.track', keyed(source.crop((109, 8, 157, 21))), [0, 2, 0, 2])
                pack.sprite('seek.fill', keyed(source.crop((109, 21, 157, 34))), [0, 2, 0, 2])
                for i in range(18): pack.sprite('volume.' + str(i), keyed(source.crop((225, i * 13, 273, (i + 1) * 13))))
            else:
                slots['seek'] = {'at': [6, 51, 6, 18], 'stretchX': True, 'fromBottom': True}
                slots['volume'] = {'at': [6, 21, 79, 14], 'fromBottom': True}
                # Preserve rounded ends; the middle is the original 15-pixel strip.
                for kind, sy in [('track',1337),('fill',1319)]:
                    lineim = Image.new('RGBA', (313,18))
                    offset=149 if kind=='track' else 0
                    lineim.paste(source.crop((offset,1319,offset+149,1337)),(0,0))
                    lineim.paste(source.crop((299,sy,314,sy+18)),(149,0))
                    lineim.paste(source.crop((offset,1337,offset+149,1355)),(164,0))
                    pack.sprite('seek.' + kind, keyed(lineim), [0,149,0,149])
                for i in range(31): pack.sprite('volume.' + str(i), keyed(source.crop((332, i * 14, 411, (i + 1) * 14))))
        else:
            slots['queue'] = {'at': [7 if not silver else 6, 29 if not silver else 21, 28 if not silver else 22, 70 if not silver else 50], 'stretchX': True, 'stretchY': True}
            slots['title'] = {'at': [30, 5, 160, 15], 'stretchX': True}
            if quicktime:
                slots['queue']={'at':[7,77,7,56],'stretchX':True,'stretchY':True}
                slots['title']={'at':[69,5,21,15],'stretchX':True}
                slots['summary']={'at':[149,13,195,15],'fromBottom':True}
    pack.write()


def anchored(xexpr, yexpr, wexpr, hexpr, variables):
    """Translate source expressions into resize anchors, preserving margins."""
    x, y, w, h = [number(v, variables) for v in (xexpr, yexpr, wexpr, hexpr)]
    slot = {'at': [x, y, w, h]}
    for axis, start, length, dimension in [(0, xexpr, wexpr, 'width'), (1, yexpr, hexpr, 'height')]:
        total = variables['win' + dimension]
        if re.search(r'<(?:pl)?win' + dimension + '>', length, re.I):
            slot['at'][axis + 2] = total - slot['at'][axis] - slot['at'][axis + 2]
            slot['stretch' + ('X' if axis == 0 else 'Y')] = True
        elif re.search(r'<(?:pl)?win' + dimension + '>', start, re.I):
            slot['at'][axis] = total - slot['at'][axis] - slot['at'][axis + 2]
            slot['fromRight' if axis == 0 else 'fromBottom'] = True
    return slot


def zoom_original(name, skn, bitmap, size, screen, slices, playlist=None, transport=None):
    """Read the default original Zoom layout; SKN commands are never executed.

    This path supports keyed copies, source timeline end caps, implicit pressed
    button states, and skins whose original main UI has no volume slider.
    """
    original = SOURCES / name / 'original'
    source = Image.open(original / bitmap).convert('RGBA')
    script = (original / skn).read_text()
    pack = Pack(name)
    actions = FUNCTIONS | {
        'fnzoom': 'compact', 'fnoptions': 'menu', 'fnskin': 'skins',
        'fnplcontrol': 'menu', 'exgroupset': 'menu', 'fnmute': 'mute',
        'fnprevtrack': 'prev', 'fnnexttrack': 'next',
        'fnaudiotrack': 'audio', 'fndvdsub': 'subtitles',
        'fnvolup': 'volup', 'fnvoldown': 'voldown',
        'fnincrate': 'faster', 'fndecrate': 'slower',
        'fnskipbackward': 'skipback', 'fnskipforward': 'skipforward',
    }
    if name == 'zoom-player-brownish':
        actions.update(fnprevchapter='chapterprev', fnnextchapter='chapternext')
    roles = [('main', size, slices, {1})]
    if playlist:
        roles.append(('playlist', playlist['size'], playlist['slices'], playlist.get('groups', {1})))
    for role, (w, h), margins, active in roles:
        lines = list(zoom_lines(script, active))
        variables = {'winwidth': w, 'winheight': h, 'winhalfwidth': w / 2,
                     'winhalfheight': h / 2, 'plwinwidth': w, 'plwinheight': h,
                     'plwinhalfwidth': w / 2,
                     'vidwidth': w - screen[0] - screen[2],
                     'vidheight': h - screen[1] - screen[3]}
        prefix = 'pl' if role == 'playlist' else ''
        face = Image.new('RGBA', (w, h))
        if role == 'playlist':
            # Some original playlists fill their well through PlayListData,
            # without a FillPLRect. Keep that region opaque in the silhouette.
            data = next((parameters(l) for l in lines if l.lower().startswith('playlistdata(')), None)
            if data:
                x, y, rw, rh = [number(data[k], variables) for k in ('destx','desty','width','height')]
                color = data.get('background', '000000').upper().removesuffix('NT')
                ImageDraw.Draw(face).rectangle((x,y,x+rw-1,y+rh-1), fill='#'+color)
        overlay = Image.new('RGBA', transport) if role == 'main' and transport else None
        for line in lines:
            m = re.fullmatch(r'(Copy|Tile|Fill)' + prefix + r'(transBitmap|Bitmap[HV]?|Rect(?:NT)?)\((.*)\)', line, re.I)
            if not m:
                continue
            operation, mode = m[1].lower(), m[2].lower()
            args = m[3].split(',')
            if operation == 'fill':
                x, y, rw, rh = [number(v, variables) for v in args[:4]]
                color = args[4].strip().upper().removesuffix('NT')
                if rw > 0 and rh > 0:
                    ImageDraw.Draw(face).rectangle((x, y, x + rw - 1, y + rh - 1), fill='#' + color)
                continue
            sx, sy, sw, sh, dx, dy = [number(v, variables) for v in args[:6]]
            crop = source.crop((sx, sy, sx + sw, sy + sh))
            target = face
            if overlay is not None and '<winhalfwidth>' in line.lower():
                target = overlay
                dx += transport[0] // 2 - w // 2
                dy += transport[1] - h
            if mode == 'transbitmap':
                crop = keyed(crop, (tuple(bytes.fromhex(args[6].strip())),))
            if operation == 'copy':
                if mode == 'transbitmap':
                    target.alpha_composite(crop, (dx, dy))
                else:
                    target.paste(crop, (dx, dy))
            else:
                length = number(args[6], variables)
                horizontal = mode.endswith('h')
                step = sw if horizontal else sh
                for offset in range(0, max(0, length), step):
                    tile = crop.crop((0, 0, min(sw, length - offset) if horizontal else sw,
                                      sh if horizontal else min(sh, length - offset)))
                    target.paste(tile, (dx + offset if horizontal else dx, dy if horizontal else dy + offset))
        pack.sprite('face.' + role, keyed(face), margins)
        slots = pack.layout('video.' + role, [w, h], 'face.' + role)
        transport_slots = None
        if overlay is not None:
            pack.sprite('face.transport', keyed(overlay))
            transport_slots = pack.layout('video.transport', list(transport), 'face.transport')
        for index, line in enumerate(lines):
            if not line.lower().startswith('createexbutton('):
                continue
            values = parameters(line)
            if values.get('target', 'main').lower() != role:
                continue
            function = values.get('function', '').lower()
            key = actions.get(function, 'ornament.' + function)
            if role == 'playlist' and key == 'playlist':
                key = 'close'
            dest = transport_slots if transport_slots is not None and '<winhalfwidth>' in values['destx'].lower() else slots
            if key in dest:
                key = 'ornament.' + function + '.' + str(index)
            bw, bh, sx, sy = [number(values[k], variables) for k in ('width', 'height', 'srcx', 'srcy')]
            # Zoom's implicit down state is immediately to the right of normal.
            values.setdefault('downx', str(sx + bw))
            values.setdefault('downy', str(sy))
            art = {}
            for state, xkey, ykey in [('normal','srcx','srcy'), ('hover','hoverx','hovery'),
                                     ('pressed','downx','downy'), ('checked','activex','activey'),
                                     ('checkedHover','activehoverx','activehovery'),
                                     ('checkedPressed','activedownx','activedowny')]:
                if xkey not in values or ykey not in values:
                    continue
                x, y = number(values[xkey], variables), number(values[ykey], variables)
                sprite = role + '.' + key + '.' + state
                pack.sprite(sprite, keyed(source.crop((x, y, x + bw, y + bh))))
                art[state] = sprite
            slot = anchored(values['destx'], values['desty'], values['width'], values['height'], variables)
            slot['art'] = art
            if dest is transport_slots:
                slot['at'][0] = number(values['destx'], variables) - w // 2 + transport[0] // 2
                slot['at'][1] = number(values['desty'], variables) - h + transport[1]
                slot.pop('fromBottom', None)
            dest[key] = slot
        if role == 'main':
            slots['screen'] = {'at': list(screen), 'stretchX': True, 'stretchY': True}
            declarations = {}
            for line in lines:
                m = re.fullmatch(r'(\w+)\s*=\s*\((.*)\)', line)
                if m:
                    declarations[m[1].lower()] = m[2]
            slots['seek'] = anchored(*(declarations['tline' + k] for k in ('left','top','width','height')), variables)
            timeline_height = number(declarations['tlineheight'], variables)
            for kind, command in [('track', 'bg'), ('fill', 'fg')]:
                args = next(re.fullmatch(r'TimeLine' + command + r'\((.*)\)', l, re.I)[1]
                            for l in lines if re.fullmatch(r'TimeLine' + command + r'\((.*)\)', l, re.I))
                x, y, tw = [number(v, variables) for v in args.split(',')]
                middle = source.crop((x, y, x + tw, y + timeline_height))
                caps = []
                for end in ('Start', 'End'):
                    matches = [re.fullmatch(r'TimeLine' + end + r'\((.*)\)', l, re.I) for l in lines]
                    match = next((m for m in matches if m), None)
                    if match:
                        cx, cy, cw = [number(v, variables) for v in match[1].split(',')]
                        if kind == 'track': cx += cw
                        caps.append(source.crop((cx, cy, cx + cw, cy + timeline_height)))
                    else:
                        caps.append(Image.new('RGBA', (0, timeline_height)))
                track = Image.new('RGBA', (caps[0].width + tw + caps[1].width, timeline_height))
                offset = 0
                for part in (caps[0], middle, caps[1]):
                    track.paste(part, (offset, 0)); offset += part.width
                pack.sprite('seek.' + kind, keyed(track), [0, caps[1].width, 0, caps[0].width])
            for line in lines:
                if line.lower().startswith('volumeexdata('):
                    values = parameters(line)
                    slots['volume'] = anchored(*(values[k] for k in ('destx','desty','width','height')), variables)
                    x, y, vw, vh, frames = [number(values[k], variables) for k in ('srcx','srcy','width','height','images')]
                    for i in range(frames):
                        row = frames - 1 - i
                        pack.sprite('volume.' + str(i), keyed(source.crop((x,y+row*vh,x+vw,y+(row+1)*vh))))
                elif line.lower().startswith('rateexdata('):
                    values = parameters(line)
                    slots['ornament.rate'] = anchored(*(values[k] for k in ('destx','desty','width','height')), variables)
                    x, y, rw, rh = [number(values[k], variables) for k in ('srcx','srcy','width','height')]
                    pack.sprite('rate.normal', keyed(source.crop((x,y,x+rw,y+rh))))
                    slots['ornament.rate']['art'] = 'rate.normal'
        else:
            values = next((parameters(l) for l in lines if l.lower().startswith('playlistdata(')), None)
            if values:
                slots['queue'] = anchored(*(values[k] for k in ('destx','desty','width','height')), variables)
                # Leave the original search/scroll region outside the rows.
                slots['queue']['at'][2] += int(values.get('scrollwidth', values.get('scrscrollwidth', 16)))
                slots['queue']['at'][3] += 20
        for line in lines:
            if not line.lower().startswith('drawextext('):
                continue
            values = parameters(line)
            if values.get('target', 'main').lower() != role or not values.get('text'):
                continue
            key = 'title' if role == 'main' or 'playlistitems' in values['text'].lower() else 'summary'
            slots[key] = anchored(*(values[k] for k in ('destx','desty','width','height')), variables)
        if role == 'playlist' and name == 'zoom-player-fusion':
            # The total-time readout has its own right-aligned well.
            slots['title'] = anchored('11','6','<PLWinWidth>-156','14', variables)
            slots['summary'] = anchored('<PLWinWidth>-130','9','73','14', variables)
    pack.write()


def bsplayer():
    source = SOURCES / 'bsplayer/original'
    pack = Pack('bsplayer')
    image = Image.open(source / 'main.bmp')
    pack.sprite('face.controller', image)
    slots = pack.layout('video.controller', list(image.size), 'face.controller')
    for key, prefix, x, y in [('open','open',9,53),('playlist','list',40,53),('pause','pause',120,53),('play','play',151,53),('stop','stop',198,53),('prev','prev',229,53),('next','next',260,53),('close','exit',780,5)]:
        art = {}; size = None
        for state, suffix in [('normal','n'),('hover','u'),('pressed','d')]:
            path = source / (prefix + suffix + '.bmp')
            if path.exists():
                im = Image.open(path); size = im.size
                sprite = 'key.' + key + '.' + state; pack.sprite(sprite, im); art[state] = sprite
        slots[key] = {'at': [x, y, *size], 'art': art}
    slots['seek'] = {'at': [7,42,786,7]}
    pack.sprite('seek.track',Image.new('RGBA',(3,7),'black'),[1,1,1,1])
    pack.sprite('seek.fill',Image.new('RGBA',(3,7),'#313A31'),[1,1,1,1])
    slots['volume'] = {'at': [466,54,152,19]}
    slots['title'] = {'at': [11,9,390,15]}
    slots['elapsed'] = {'at': [410,9,185,18]}
    slots['status'] = {'at': [624,9,150,18]}
    volume = Image.open(source / 'volume.bmp')
    pack.sprite('volume.track',image.crop((466,54,618,73)))
    pack.sprite('volume.fill',keyed(volume,((128,128,128),)))
    pack.write()


def vlc():
    source = SOURCES / 'vlc/original'
    root = ET.parse(source / 'theme.xml').getroot()
    images = {}
    for bitmap in root.findall('Bitmap'):
        color = bitmap.get('alphacolor', '#FF00FF')
        key = tuple(bytes.fromhex(color.removeprefix('#')))
        im = keyed(Image.open(source / bitmap.get('file')), (key,))
        images[bitmap.get('id')] = im
        for sub in bitmap:
            x,y,w,h = [int(sub.get(k)) for k in ('x','y','width','height')]
            images[sub.get('id')] = im.crop((x,y,x+w,y+h))
    pack = Pack('vlc')
    actions = {'vlc.quit()':'close','vlc.minimize()':'minimize','vlc.play()':'play',
               'playlist.previous()':'prev','playlist.next()':'next','vlc.stop()':'stop',
               'vlc.slower()':'slower','vlc.faster()':'faster','dialogs.file()':'open',
               'dialogs.preferences()':'skins','dialogs.fileInfo()':'menu',
               'plwin.show()':'playlist','eqwin.show()':'audio','vlc.mute()':'mute',
               'playlist.setRandom(true)':'shuffle','playlist.setLoop(true)':'repeat',
               'dialogs.playlist()':'playlist','dialogs.addFile()':'add',
               'playlist.delete()':'remove','vlc.fullscreen()':'full',
               'playlist.add()':'add','playtree.del()':'remove','playtree.sort()':'sort',
               'playlist.load()':'load','playlist.save()':'save'}
    for role, win, face, slices in [('main','main','main_blank',[24,25,96,25]), ('playlist','plwin','playlist_bg',[28,30,47,14])]:
        layout = root.find("Window[@id='"+win+"']/Layout")
        w,h = int(layout.get('width')),int(layout.get('height'))
        pack.sprite('face.'+role,images[face].crop((0,0,w,h)),slices)
        slots = pack.layout('video.'+role,[w,h],'face.'+role)
        for e in layout.iter():
            if e.tag not in ('Button','Checkbox'): continue
            action = e.get('action',e.get('action1',''))
            key = actions.get(action)
            if action == 'plwin.hide()': key = 'close'
            if action.startswith('minwin.show'): key = 'compact'
            if action == 'dialogs.changeSkin()': key = 'skins'
            if action == 'vlc.toggleFullscreen()': key = 'full'
            if action == 'playlist.del()': key = 'remove'
            if not key or key in slots or 'dvd.' in e.get('visible',''): continue
            art = {}
            for state,attr in [('normal','up'),('hover','over'),('pressed','down'),('checked','up2'),('checkedHover','over2'),('checkedPressed','down2')]:
                img = e.get(attr, e.get(attr+'1'))
                if img not in images: continue
                pack.sprite(role+'.'+key+'.'+state,images[img]); art[state] = role+'.'+key+'.'+state
            im = images[e.get('up',e.get('up1'))]
            x,y = int(e.get('x','0')),int(e.get('y','0'))
            slot = {'at':[x,y,*im.size],'art':art}
            if e.get('lefttop','').startswith('right'): slot['fromRight']=True;slot['at'][0]=w-x-im.width
            if e.get('lefttop','').endswith('bottom'): slot['fromBottom']=True;slot['at'][1]=h-y-im.height
            slots[key]=slot
        if role == 'main':
            slots['screen']={'at':[21,24,22,96],'stretchX':True,'stretchY':True}
            slots['title']={'at':[21,75,114,18],'stretchX':True,'fromBottom':True}
            slots['elapsed']={'at':[24,74,88,16],'fromRight':True,'fromBottom':True}
            slots['seek']={'at':[21,65,22,15],'stretchX':True,'fromBottom':True}
            slots['volume']={'at':[310,21,75,20],'fromBottom':True}
            pack.sprite('seek.thumb',images['timeslider'])
            pack.sprite('volume.thumb',images['vol_slider'])
            bg=images['vol_bg'];fh=bg.height//55
            for i in range(55):pack.sprite('volume.'+str(i),bg.crop((0,i*fh,bg.width,(i+1)*fh)))
        else: slots['queue']={'at':[14,27,30,46],'stretchX':True,'stretchY':True}
    pack.write()


def powerdvd():
    source=SOURCES/'powerdvd/original'
    # Read the standard panel, ignoring the independent tiny-mode declarations.
    raw=(source/'skin.txt').read_text(encoding='latin1').split('<Panel>',1)[1]
    mapping_raw=(source/'skin.txt').read_text(encoding='latin1').split('<Bitmaps>')[-1]
    maps=dict(re.findall(r'^(BMP\d+)=(.*)',mapping_raw,re.M))
    sections={}
    for title,body in re.findall(r'<([^>]+)>\s*([^<]*)', '<Panel>'+raw):
        sections[title.strip()] = dict(re.findall(r'^([A-Za-z_]+)=(.*)',body,re.M))
    def crop(value):
        parts=value.strip().split(',');filename=maps[parts[0]].strip().split(',')[0].upper()+'.bmp'
        im=keyed(Image.open(source/filename),((255,0,255),))
        if len(parts)==5:
            x,y,r,b=map(int,parts[1:]);im=im.crop((x,y,r+1,b+1))
        return im
    pack=Pack('powerdvd')
    panel=sections['Panel'];im=crop(panel['BMP_UP']).crop((0,0,422,124))
    mask=Image.new('L',im.size);values=list(map(int,panel['REGION'].split(',')))
    ImageDraw.Draw(mask).polygon(list(zip(values[::2],values[1::2])),fill=255);im.putalpha(mask)
    pack.sprite('face.controller',im)
    slots=pack.layout('video.controller',list(im.size),'face.controller')
    for title,key in [('Play','play'),('Stop','stop'),('Pause','pause'),('Begin','prev'),('End','next'),('Mute','mute'),('Zoom','full'),('Config','menu'),('MenuList','playlist'),('Power','close'),('Min','minimize'),('SelectSource','open'),('SwitchSkin','skins'),('VolInc','volup'),('VolDec','voldown')]:
        section=sections.get(title,{})
        if 'POSITION' not in section or not section.get('BMP_UP'):continue
        x,y=map(int,section['POSITION'].strip().split(','));art={}
        for field,state in [('BMP_UP','normal'),('BMP_SHINE','hover'),('BMP_DOWN','pressed'),('BMP_GRAY','disabled')]:
            if not section.get(field):continue
            image=crop(section[field]);sprite=key+'.'+state;pack.sprite(sprite,image);art[state]=sprite
        normal=crop(section['BMP_UP']);slots[key]={'at':[x,y,*normal.size],'art':art}
    slots['volume']={'at':[39,16,19,81]}
    volume=Image.open(source/'VOLIMAGE.BMP.bmp')
    for i in range(11):pack.sprite('volume.'+str(i),volume.crop((i*19,0,(i+1)*19,81)))
    slots['seek']={'at':[46,105,205,8]}
    pack.sprite('seek.track',Image.new('RGBA',(3,8),'#555555'),[1,1,1,1])
    pack.sprite('seek.fill',Image.new('RGBA',(3,8),'#BCD238'),[1,1,1,1])
    slots['elapsed']={'at':[71,44,140,24]}
    pack.write()


if __name__ == '__main__':
    zoom('zoom-player', 'Onyx.skn', 'Onyx.bmp', 638, 454)
    zoom('zoom-player-silver', 'Silverchrome.skn', 'Silverchrome.bmp', 412, 363, silver=True)
    bsplayer()
    vlc()
    powerdvd()
    zoom('quicktime','iFix v040611 (standard).skn','iFix 040611.bmp',360,370,quicktime=True)
    zoom_original('zoom-player-fusion', 'Fusion.skn', 'Fusion.bmp', (618,427),
                  (6,29,6,57), [32,139,57,85],
                  playlist={'size': (450,214), 'slices': [32,139,70,85]}, transport=(276,40))
    zoom_original('zoom-player-gtz-hd', 'GTZ HD.skn', 'GTZ-HD.bmp', (300,282),
                  (7,34,7,72), [35,134,71,50],
                  playlist={'size': (232,186), 'slices': [8,10,8,10], 'groups': {2}}, transport=(276,45))
    zoom_original('zoom-player-brownish', 'brownish.skn', 'brownish.bmp', (430,360),
                  (4,4,26,56), [4,199,56,21])
