# ==========================================
# 01_basics.nl: Core Syntax in NeuroLang
# ==========================================

# 1. Variables & Literals
x = 42
rate = 0.15
title = "NeuroLang AI Runtime"
is_active = true

print("Variables initialized:")
print("x:", x, "rate:", rate, "title:", title)

# 2. Lists & Maps
scores = [95, 82, 67, 99, 88]
user = {
  id: 101,
  name: "Alice",
  role: "agent",
  skills: ["search", "code", "deploy"]
}

print("User profile:", user.name, "with role:", user.role)
print("Skills count:", len(user.skills))

# 3. Functions & Lambdas
square = n -> n * n
multiply = (a, b) -> a * b

print("Square of 8:", square(8))
print("Multiply 7 * 6:", multiply(7, 6))

# 4. Pattern Matching
priority = match user.role {
  "admin" -> "HIGH"
  "agent" -> "MEDIUM"
  _ -> "LOW"
}
print("Assigned priority:", priority)
