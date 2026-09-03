# ==========================================
# 02_dataflow.nl: Dataflow Pipelines & Streams
# ==========================================

# Mock dataset of customer orders
orders = [
  {id: "ord-1", customer: "Alice", amount: 150, status: "completed"},
  {id: "ord-2", customer: "Bob", amount: 45, status: "pending"},
  {id: "ord-3", customer: "Charlie", amount: 320, status: "completed"},
  {id: "ord-4", customer: "Diana", amount: 80, status: "cancelled"},
  {id: "ord-5", customer: "Eve", amount: 210, status: "completed"}
]

# Pipeline 1: Filter completed orders above 100
premium_orders = orders | ?(.status == "completed" && .amount >= 100)
print("Premium orders:", premium_orders)

# Pipeline 2: Project with calculated 20% tax and format
taxed_orders = orders
  | ?(.status == "completed")
  | @{id: .id, client: .customer, total: .amount * 1.2}

print("Taxed orders summary:", taxed_orders)

# Pipeline 3: Extract amounts and sum via reduce
add = (a, b) -> a + b
total_revenue = orders
  | ?(.status == "completed")
  | @.amount
  | &(add)

print("Total revenue from completed orders: $", total_revenue)
