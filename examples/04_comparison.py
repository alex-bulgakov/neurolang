# ==========================================
# 04_comparison.py: Equivalent Python Task
# ==========================================

def process_orders(orders):
    result = []
    for order in orders:
        if order.get("status") == "active":
            total = order.get("amount", 0) * 1.2
            if total > 100:
                result.append({"id": order.get("id"), "total": total})
    return result

orders = [
    {"id": 1, "amount": 120, "status": "active"},
    {"id": 2, "amount": 40, "status": "pending"},
    {"id": 3, "amount": 90, "status": "active"}
]

result = process_orders(orders)
print(result)
