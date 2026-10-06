"""Exhaustive algebra check for the ascal uint12 end-of-line condition.

VHDL uint12 is an integer subtype (0..4095), so intermediate h+1 can be
4096; it must not wrap before comparison. Python's positive-divisor modulo
matches VHDL MOD, including (0-1) MOD 4096 == 4095.
This proves the decision for every pair, even during timing-mode changes;
it is not a simulation of the complete scaler.
"""

for total in range(4096):
    last = (total - 1) % 4096
    for horizontal in range(4096):
        original = horizontal + 1 < total
        shortened = horizontal < total and horizontal != last
        if original != shortened:
            raise AssertionError((horizontal, total, original, shortened))

print("PASS: scaler line-end comparison equivalent for all 16,777,216 uint12 pairs")
