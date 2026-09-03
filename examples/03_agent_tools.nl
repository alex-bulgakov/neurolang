# ==========================================
# 03_agent_tools.nl: External Agent Tools & MCP
# ==========================================

# 1. Inspect filesystem via !fs.list
files = !fs.list("examples")
print("Files in examples directory:")
print(files)

# 2. Filter only .nl files
nl_files = files | ?(. != "temp")
print("Filtered scripts:", nl_files)

# 3. Create a temporary report file
report = {
  timestamp: !time.now(),
  agent: "NeuroLang-Core",
  status: "healthy",
  modules_scanned: len(files)
}

!fs.write("temp_report.json", report)
print("Wrote JSON report to temp_report.json")

# 4. Read back and verify
saved_data = !fs.read("temp_report.json")
print("Read back report:", saved_data.agent, "status:", saved_data.status)
