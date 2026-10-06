# Compatibility for legacy MUGEN 1.0 screenpacks on the fixed 4:3 Frontier output.
# Windows MUGEN's width-based canvas can extend below the declared localcoord
# height. Make that canvas explicit so IKEMEN does not height-fit and crop it.
# Only the temporary overlay motif is replaced; source assets are never edited.
{
    lines[NR] = $0
    value = $0
    sub(/;.*/, "", value)
    gsub(/\r/, "", value)
    if (value ~ /^[ \t]*\[/) {
        section = tolower(value)
        gsub(/[ \t]/, "", section)
    }
    if (section != "[info]") next
    equal = index(value, "=")
    if (!equal) next
    key = tolower(substr(value, 1, equal - 1))
    gsub(/[ \t]/, "", key)
    value = substr(value, equal + 1)
    gsub(/[ \t"]/, "", value)
    if (key == "mugenversion") version = value
    if (key == "ikemenversion") ikemen = value
    if (key == "localcoord") {
        split(value, coord, ",")
        width = coord[1] + 0
        height = coord[2] + 0
        coordLine = NR
    }
}
END {
    if (version == "1.0" && ikemen == "" && width > 0 && height > 0 && width * 3 > height * 4) {
        lines[coordLine] = "localcoord = " width "," int(width * 3 / 4 + 0.5) " ; Frontier MUGEN width-coordinate compatibility"
    }
    for (i = 1; i <= NR; i++) print lines[i]
}
