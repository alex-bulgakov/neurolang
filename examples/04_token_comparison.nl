# ==========================================
# 04_token_comparison.nl: AI Agent Task
# ==========================================
# Task: Given orders, filter active, add tax, filter total > 100

orders = [
  {id: 1, amount: 120, status: "active"},
  {id: 2, amount: 40, status: "pending"},
  {id: 3, amount: 90, status: "active"}
]

result = orders | ?(.status == "active") | @{id: .id, total: .amount * 1.2} | ?(.total > 100)
print(result)
