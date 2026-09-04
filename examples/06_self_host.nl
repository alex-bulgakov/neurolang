# ==========================================================
# 06_self_host.nl — neurolang run examples/06_self_host.nl
# The Go host only bootstraps std/; this program is evaluated by NL-in-NL.
# ==========================================================

nums = [1, 2, 3, 4, 5]
grown = nums | ?(. > 2) | @(. * 10)
total = grown | &((a, b) -> a + b)

label = match total {
  120 -> "ok"
  _ -> "bad"
}

print("grown:", grown)
print("total:", total)
print("label:", label)
