import copy
import struct
import io
from contextlib import redirect_stdout
from pathlib import Path
from tempfile import TemporaryDirectory
from types import SimpleNamespace
import unittest

import release_artifacts as release


class PrivacyChecks(unittest.TestCase):
    def setUp(self):
        self.files = {name: (release.ROOT / name).read_bytes()
                      for name in release.allowed_payload() - release.BASE_FILES}
        for name in ("LICENSE", "THIRD_PARTY_NOTICES.md"):
            self.files[name] = (release.ROOT / name).read_bytes()
        self.files["使用指南.md"] = (release.ROOT / "docs/USAGE.md").read_bytes()
        self.files["data/games.json"] = release.json.dumps(release.EMPTY_LIBRARY).encode()
        exe = bytearray(256)
        exe[:2] = b"MZ"
        struct.pack_into("<I", exe, 0x3c, 128)
        exe[128:132] = b"PE\0\0"
        struct.pack_into("<H", exe, 132, 0x8664)
        self.files["游栖.exe"] = bytes(exe)

    def test_private_source_paths_are_rejected(self):
        for name in ("games.json", "desktop/data/games.json", "desktop/data/integrations.json",
                     ".env", ".env.production", "backup/preferences.json", "icons/private.png",
                     "credentials.key", "desktop/.tools/tool.exe"):
            with self.subTest(name=name):
                self.assertTrue(release.private_path(name))
        self.assertFalse(release.private_path("desktop/data.example/games.json"))
        self.assertFalse(release.private_path("docs/screenshots/main-steam.jpg"))

    def test_empty_allowlisted_payload_is_accepted(self):
        release.check_payload(self.files)

    def test_extra_configuration_file_is_rejected(self):
        self.files["data/integrations.json"] = b'{"key":"fixture-not-a-real-key"}'
        with self.assertRaises(ValueError):
            release.check_payload(self.files)

    def test_games_or_covert_fields_are_rejected(self):
        for change in ({"games": [{"name": "Private fixture"}]}, {"key": "fixture"}, {"updated": "private"}):
            with self.subTest(change=change):
                library = copy.deepcopy(release.EMPTY_LIBRARY)
                library.update(change)
                self.files["data/games.json"] = release.json.dumps(library).encode()
                with self.assertRaises(ValueError):
                    release.check_payload(self.files)

    def test_replaced_documents_are_rejected(self):
        self.files["LICENSE"] = b"unexpected personal file"
        with self.assertRaises(ValueError):
            release.check_payload(self.files)

    def test_invalid_versions_are_rejected(self):
        for version in ("v1.0.0", "1.0", "1.0.0/../../", "01.0.0", "1.0.0-rc.1"):
            with self.assertRaises(ValueError):
                release.names(version)

    def test_full_artifact_check_rejects_extra_configuration_even_with_updated_checksums(self):
        with TemporaryDirectory() as tmp:
            root = Path(tmp)
            payload = root / "payload"
            for name, content in self.files.items():
                path = payload / name
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_bytes(content)
            bootstrapper = root / "bootstrapper.exe"
            bootstrapper.write_bytes(b"fixture-bootstrapper")
            raw = b"".join(struct.pack("<I", len(b)) + b for b in list(self.files.values()) + [bootstrapper.read_bytes()])
            compressor = release.zlib.compressobj(wbits=-15)
            installer = root / "installer.exe"
            installer.write_bytes(bytes(4) + bytes.fromhex("efbeadde") + b"NullsoftInst" + bytes(8)
                                  + compressor.compress(raw) + compressor.flush() + bytes(4))
            args = SimpleNamespace(output=str(root / "dist"), payload=str(payload), installer=str(installer),
                                   bootstrapper=str(bootstrapper), version="1.2.3", commit="a" * 40)
            with redirect_stdout(io.StringIO()):
                release.package(args)
                release.verify(args)
            # Simulate a packaging regression and recompute checksums: content policy must still fail.
            dist = Path(args.output)
            zip_name, installer_name = release.names(args.version)
            with release.zipfile.ZipFile(dist / zip_name, "a") as archive:
                archive.writestr("GameNest/data/integrations.json", b'{"fixture":"not-a-real-key"}')
            manifest = release.json.loads((dist / "release-manifest.json").read_text(encoding="utf-8"))
            manifest["assets"][zip_name] = release.digest((dist / zip_name).read_bytes())
            (dist / "release-manifest.json").write_bytes(release.json.dumps(manifest).encode())
            sums = "".join(release.digest((dist / n).read_bytes()) + "  " + n + "\n"
                           for n in (zip_name, installer_name, "release-manifest.json"))
            (dist / "SHA256SUMS.txt").write_bytes(sums.encode("ascii"))
            with self.assertRaisesRegex(ValueError, "allowlist"):
                release.verify(args)


if __name__ == "__main__":
    unittest.main()
