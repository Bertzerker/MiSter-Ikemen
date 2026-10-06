#!/usr/bin/env python3
"""Audit unpacked MUGEN assets. Default is read-only; --fix-case backs up edits."""
import argparse
import collections
import datetime
import hashlib
import json
import os
from pathlib import Path
import posixpath
import re
import shutil

TEXT = {'.def', '.cfg', '.ini'}
ASSETS = {'.def', '.sff', '.snd', '.air', '.cmd', '.cns', '.st', '.fnt',
          '.ttf', '.otf', '.mp3', '.ogg', '.wav', '.flac', '.pcx', '.png'}
LINE = re.compile(r'^(\s*[^;=\r\n]+?\s*=\s*)("[^"\r\n]*"|[^;,\r\n]*)(.*)$')
KEY = re.compile(r'^(?:spr|sprite|anim|cmd|cns|st\d*|stcommon|snd|sound|pal\d+|font\d*(?:\.file)?|file|fight|select|motif|logo|intro|ending|bgmusic|music|.*\.(?:bgm|storyboard))$', re.I)

def inventory(root):
    paths = []
    for base, dirs, files in os.walk(root, followlinks=False):
        dirs[:] = sorted(d for d in dirs if not d.startswith('.mugen-repair-')
                         and not (Path(base) / d).is_symlink())
        for name in sorted(files):
            p = Path(base) / name
            if not p.is_symlink():
                paths.append(p.relative_to(root).as_posix())
    return paths

def audit(root):
    root = Path(root).resolve(strict=True)
    paths = inventory(root)
    folded = collections.defaultdict(list)
    for path in paths:
        folded[path.casefold()].append(path)
    issues, edits = [], {}
    for name in paths:
        if Path(name).suffix.lower() not in TEXT:
            continue
        original = (root / name).read_bytes()
        # Latin-1 is a lossless byte mapping; preserve legacy encodings and CRLF.
        lines = original.decode('latin-1').splitlines(keepends=True)
        changed = False
        for number, line in enumerate(lines, 1):
            match = LINE.match(line.rstrip('\r\n'))
            if not match:
                continue
            key = match[1].split('=', 1)[0].strip()
            if not KEY.fullmatch(key):
                continue
            token = match[2].strip()
            quoted = token.startswith('"') and token.endswith('"')
            ref = token[1:-1] if quoted else token
            if Path(ref.replace('\\', '/')).suffix.lower() not in ASSETS:
                continue
            normalized = ref.replace('\\', '/')
            if normalized.startswith('/') or re.match(r'^[A-Za-z]:', normalized):
                issues.append(dict(file=name, line=number, reference=ref, status='external-path'))
                continue
            parent = posixpath.dirname(name)
            candidates = []
            for base in (parent, '', 'data', 'font', 'sound'):
                candidate = posixpath.normpath(posixpath.join(base, normalized))
                if candidate == '..' or candidate.startswith('../'):
                    continue
                for actual in folded.get(candidate.casefold(), []):
                    if actual not in [a for _, a in candidates]:
                        candidates.append((base, actual))
            if not candidates:
                status, replacement = 'unresolved', None
            elif len(candidates) != 1:
                status, replacement = 'ambiguous', None
            else:
                base, actual = candidates[0]
                replacement = posixpath.relpath(actual, base or '.')
                # Only patch explicit paths resolved in the referencing folder
                # or game root. Engine-specific fallback roots are report-only.
                same = posixpath.normpath(posixpath.join(base, normalized)) == actual
                if same:
                    continue
                status = 'case-mismatch' if base in (parent, '') else 'case-mismatch-review'
            issue = dict(file=name, line=number, reference=ref, status=status)
            if candidates:
                issue['candidates'] = [a for _, a in candidates]
            if replacement is not None:
                issue['replacement'] = replacement
            issues.append(issue)
            if status == 'case-mismatch':
                value = ('"' + replacement + '"') if quoted else replacement
                rhs = match[2]
                start = len(rhs) - len(rhs.lstrip())
                end = len(rhs.rstrip())
                newline = line[len(line.rstrip('\r\n')):]
                lines[number-1] = match[1] + rhs[:start] + value + rhs[end:] + match[3] + newline
                changed = True
        if changed:
            edits[name] = (original, ''.join(lines).encode('latin-1'))
    report = dict(root=str(root), files=len(paths), issues=issues,
                  case_collisions=[v for v in folded.values() if len(v) > 1],
                  music=collections.Counter(Path(p).suffix.lower() for p in paths
                                            if Path(p).suffix.lower() in {'.mp3','.ogg','.wav','.flac'}),
                  proposed_files=[dict(file=n, sha256=hashlib.sha256(v[0]).hexdigest())
                                  for n,v in edits.items()],
                  limitations=['Unpacked .def/.cfg/.ini assignments only; scripts, ZIPs and bare select.def character entries are not parsed.',
                               'Unresolved references may use engine-specific search roots; they are not proof of missing files.',
                               'Does not validate font rendering, executable compatibility or music loop metadata.'])
    return report, edits

def apply(root, edits):
    root = Path(root).resolve(strict=True)
    # Validate all sources before writing any changes.
    for name, (original, _) in edits.items():
        path = (root / name).resolve(strict=True)
        path.relative_to(root)
        if path.read_bytes() != original:
            raise RuntimeError('File changed during audit: ' + name)
    if not edits:
        return None
    backup = root / ('.mugen-repair-' + datetime.datetime.now().strftime('%Y%m%d-%H%M%S-%f'))
    backup.mkdir()
    for name, (original, _) in edits.items():
        dest = backup / name
        dest.parent.mkdir(parents=True, exist_ok=True)
        dest.write_bytes(original)
    for name, (_, updated) in edits.items():
        path = root / name
        temporary = path.with_name(path.name + '.mugen-repair-tmp')
        with temporary.open('xb') as out:
            out.write(updated)
        shutil.copymode(path, temporary)
        os.replace(temporary, path)
    return str(backup)

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('game', type=Path)
    parser.add_argument('--report', type=Path, required=True)
    parser.add_argument('--fix-case', action='store_true', help='Apply unambiguous explicit-path case fixes with backups')
    args = parser.parse_args()
    report, edits = audit(args.game)
    if args.fix_case:
        report['backup'] = apply(args.game, edits)
    args.report.write_text(json.dumps(report, indent=2, ensure_ascii=True), encoding='utf-8')
    print(json.dumps(dict(files=report['files'], issues=dict(collections.Counter(i['status'] for i in report['issues'])),
                          repairable_files=len(edits), applied=args.fix_case), indent=2))

if __name__ == '__main__':
    main()
