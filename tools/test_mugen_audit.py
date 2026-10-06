import importlib.util
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('audit', Path(__file__).with_name('mugen-audit.py'))
audit = importlib.util.module_from_spec(spec)
spec.loader.exec_module(audit)

class AuditTests(unittest.TestCase):
    def test_lossless_fix_backup_and_ambiguity(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root/'data').mkdir(); (root/'font').mkdir()
            (root/'font'/'Menu.FNT').write_bytes(b'font')
            source = b';\xff comment\r\nfont1 = "../FONT/menu.fnt" ; keep\r\nsnd = missing.snd\r\n'
            source += b'displayname = ../FONT/menu.fnt\r\n'
            (root/'data'/'system.def').write_bytes(source)
            report, edits = audit.audit(root)
            self.assertEqual([i['status'] for i in report['issues']], ['case-mismatch','unresolved'])
            self.assertEqual((root/'data'/'system.def').read_bytes(), source)
            backup = audit.apply(root, edits)
            self.assertEqual((Path(backup)/'data'/'system.def').read_bytes(), source)
            self.assertEqual((root/'data'/'system.def').read_bytes(), source.replace(b'../FONT/menu.fnt', b'../font/Menu.FNT', 1))
            self.assertFalse(audit.audit(root)[1])
            (root/'data'/'Menu.FNT').write_bytes(b'other')
            (root/'data'/'system.def').write_bytes(b'font1 = menu.fnt\n')
            report, edits = audit.audit(root)
            self.assertEqual(report['issues'][0]['status'], 'ambiguous')
            self.assertFalse(edits)

    def test_changed_source_rejected(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp); (root/'A.snd').write_bytes(b'')
            (root/'system.def').write_bytes(b'snd=a.snd\n')
            _, edits = audit.audit(root)
            (root/'system.def').write_bytes(b'; changed\n')
            with self.assertRaises(RuntimeError): audit.apply(root, edits)

if __name__ == '__main__': unittest.main()
