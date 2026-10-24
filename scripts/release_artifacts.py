"""Allowlisted Windows release packaging and verification; no third-party packages."""
import argparse
import hashlib
import json
from pathlib import Path, PurePosixPath
import re
import struct
import subprocess
import zipfile
import zlib

ROOT = Path(__file__).resolve().parents[1]
EMPTY_LIBRARY = {"updated": "", "games": [], "note": "请在设置中选择游戏目录，然后扫描游戏。"}
BASE_FILES = {"游栖.exe", "LICENSE", "THIRD_PARTY_NOTICES.md", "使用指南.md", "data/games.json"}
PRIVATE_NAMES = {"games.json", "metadata.json", "integrations.json", "preferences.json", "backgrounds.json", "inventory.txt", "last-enrichment.json"}
PRIVATE_DIRS = {"data", "covers", "backgrounds", "icons", "node_modules", ".tools", ".installer-stage", "__pycache__"}


def require(condition, message):
    if not condition:
        raise ValueError(message)


def digest(data):
    return hashlib.sha256(data).hexdigest()


def private_path(name):
    if name == "desktop/data.example/games.json":
        return False
    path = PurePosixPath(name.lower())
    return (path.name in PRIVATE_NAMES or bool(set(path.parts) & PRIVATE_DIRS)
            or path.name == ".env" or path.name.startswith(".env.")
            or path.suffix in {".key", ".pem", ".pfx", ".p12", ".exe", ".zip"})


def check_source():
    names = subprocess.check_output(["git", "ls-files", "-z"], cwd=ROOT).decode("utf-8").split("\0")
    forbidden = [n for n in names if n and private_path(n)]
    require(not forbidden, "Private/generated files are tracked: " + ", ".join(forbidden))
    template = json.loads((ROOT / "desktop/data.example/games.json").read_text(encoding="utf-8"))
    require(template == EMPTY_LIBRARY, "Empty-library template contains unexpected data")
    print("Source privacy check: PASS")


def allowed_payload():
    licenses = {p.relative_to(ROOT).as_posix() for p in (ROOT / "licenses").rglob("*") if p.is_file()}
    return BASE_FILES | licenses


def check_payload(files):
    require(set(files) == allowed_payload(), "Payload differs from the explicit file allowlist")
    require(json.loads(files["data/games.json"].decode("utf-8")) == EMPTY_LIBRARY,
            "Payload contains non-empty or unexpected game data")
    for name in files:
        require(not (name != "data/games.json" and PurePosixPath(name).name in PRIVATE_NAMES),
                "Private configuration in payload: " + name)
    for name in allowed_payload() - {"游栖.exe", "data/games.json", "使用指南.md"}:
        require(files[name] == (ROOT / name).read_bytes(), "Unexpected document/license content: " + name)
    require(files["使用指南.md"] == (ROOT / "docs/USAGE.md").read_bytes(), "Unexpected usage guide")
    exe = files["游栖.exe"]
    require(exe[:2] == b"MZ" and len(exe) > 64, "Missing Windows executable")
    offset = struct.unpack_from("<I", exe, 0x3c)[0]
    require(exe[offset:offset + 4] == b"PE\0\0" and struct.unpack_from("<H", exe, offset + 4)[0] == 0x8664,
            "Application is not Windows amd64")


def check_installer(installer, files, bootstrapper_hash):
    # Inspect the actual NSIS solid-zlib stream without executing the installer.
    signature = installer.find(bytes.fromhex("efbeadde") + b"NullsoftInst")
    require(signature > 0, "Missing NSIS header")
    decoder = zlib.decompressobj(-15)
    raw = decoder.decompress(installer[signature + 24:-4], 128 * 1024 * 1024)
    require(decoder.eof and not decoder.unconsumed_tail, "Invalid/oversized NSIS stream")
    blocks = set()
    offset = 0
    while offset < len(raw):
        require(offset + 4 <= len(raw), "Truncated NSIS block")
        size = struct.unpack_from("<I", raw, offset)[0]
        offset += 4
        require(size <= len(raw) - offset, "Truncated NSIS file")
        blocks.add(digest(raw[offset:offset + size]))
        offset += size
    require(all(digest(content) in blocks for content in files.values()), "Installer payload differs from portable ZIP")
    require(bootstrapper_hash in blocks, "Installer does not contain the verified WebView2 bootstrapper")


def names(version):
    require(re.fullmatch(r"(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)", version), "Expected version major.minor.patch")
    prefix = "GameNest-v" + version + "-windows-amd64"
    return prefix + ".zip", prefix + "-setup.exe"


def package(args):
    check_source()
    output = Path(args.output)
    require(not output.exists(), "Output directory must be new")
    payload = Path(args.payload)
    files = {p.relative_to(payload).as_posix(): p.read_bytes() for p in payload.rglob("*") if p.is_file()}
    check_payload(files)
    zip_name, installer_name = names(args.version)
    installer = Path(args.installer).read_bytes()
    bootstrapper_hash = digest(Path(args.bootstrapper).read_bytes())
    check_installer(installer, files, bootstrapper_hash)
    output.mkdir(parents=True)
    with zipfile.ZipFile(output / zip_name, "w", zipfile.ZIP_DEFLATED, compresslevel=9) as archive:
        for name, content in sorted(files.items()):
            archive.writestr("GameNest/" + name, content)
    (output / installer_name).write_bytes(installer)
    manifest = {"version": args.version, "commit": args.commit, "webview2_sha256": bootstrapper_hash,
                "payload": {n: digest(b) for n, b in sorted(files.items())},
                "assets": {n: digest((output / n).read_bytes()) for n in (zip_name, installer_name)}}
    (output / "release-manifest.json").write_bytes((json.dumps(manifest, ensure_ascii=False, indent=2) + "\n").encode("utf-8"))
    sums = "".join(digest((output / n).read_bytes()) + "  " + n + "\n" for n in (zip_name, installer_name, "release-manifest.json"))
    (output / "SHA256SUMS.txt").write_bytes(sums.encode("ascii"))
    verify(args)


def verify(args):
    output = Path(args.output)
    zip_name, installer_name = names(args.version)
    require(re.fullmatch(r"[0-9a-f]{40}", args.commit), "Expected source commit SHA")
    expected = {zip_name, installer_name, "SHA256SUMS.txt", "release-manifest.json"}
    require({p.name for p in output.iterdir()} == expected and all((output / n).is_file() for n in expected),
            "Release directory contains unexpected files")
    manifest = json.loads((output / "release-manifest.json").read_text(encoding="utf-8"))
    require(manifest["version"] == args.version and manifest["commit"] == args.commit, "Release version/commit mismatch")
    expected_sums = "".join(digest((output / n).read_bytes()) + "  " + n + "\n" for n in (zip_name, installer_name, "release-manifest.json"))
    require((output / "SHA256SUMS.txt").read_bytes() == expected_sums.encode("ascii"), "Release checksum mismatch")
    require(manifest["assets"] == {n: digest((output / n).read_bytes()) for n in (zip_name, installer_name)}, "Asset manifest mismatch")
    with zipfile.ZipFile(output / zip_name) as archive:
        require(archive.testzip() is None, "Corrupt ZIP")
        entries = archive.namelist()
        require(len(entries) == len(set(entries)) and all(n.startswith("GameNest/") and ".." not in PurePosixPath(n).parts for n in entries), "Invalid ZIP paths")
        files = {n[len("GameNest/"):]: archive.read(n) for n in entries}
    check_payload(files)
    require(manifest["payload"] == {n: digest(b) for n, b in files.items()}, "Payload manifest mismatch")
    check_installer((output / installer_name).read_bytes(), files, manifest["webview2_sha256"])
    print("Release verification: PASS (empty data, file allowlist, matching ZIP/installer, SHA-256)")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    commands.add_parser("source")
    for command in ("package", "verify"):
        sub = commands.add_parser(command)
        for option in ("output", "version", "commit"):
            sub.add_argument("--" + option, required=True)
        if command == "package":
            for option in ("payload", "installer", "bootstrapper"):
                sub.add_argument("--" + option, required=True)
    args = parser.parse_args()
    if args.command == "source":
        check_source()
    else:
        {"package": package, "verify": verify}[args.command](args)
