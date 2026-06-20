import re

file_path = r'c:\Users\Yeikxz\Desktop\Viewred\internal\storage\queries_test.go'
with open(file_path, 'r', encoding='utf-8') as f:
    content = f.read()

# Replace sqlmock.AnyArg() * 5 for qPackets with sqlmock.AnyArg() * 7
content = re.sub(
    r'(mock\.ExpectQuery\(qPackets\)\.\n\s*WithArgs\()sqlmock\.AnyArg\(\), sqlmock\.AnyArg\(\), sqlmock\.AnyArg\(\), sqlmock\.AnyArg\(\), sqlmock\.AnyArg\(\)',
    r'\1sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()',
    content
)

# Replace sqlmock.AnyArg() * 5 for qPacketsCount with sqlmock.AnyArg() * 7
content = re.sub(
    r'(mock\.ExpectQuery\(qPacketsCount\)\.\n\s*WithArgs\()sqlmock\.AnyArg\(\), sqlmock\.AnyArg\(\), sqlmock\.AnyArg\(\), sqlmock\.AnyArg\(\), sqlmock\.AnyArg\(\)',
    r'\1sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()',
    content
)

# Replace the specific case in TestQueryPackets_PassesAllFiltersToSQL
content = content.replace(
    'WithArgs(since, until, "TCP", "10.0.0.5", "10.0.0.6", 50, 0)',
    'WithArgs(since, until, "TCP", "10.0.0.5", "10.0.0.6", 0, "", 50, 0)'
)
content = content.replace(
    'WithArgs(since, until, "TCP", "10.0.0.5", "10.0.0.6")',
    'WithArgs(since, until, "TCP", "10.0.0.5", "10.0.0.6", 0, "")'
)

with open(file_path, 'w', encoding='utf-8') as f:
    f.write(content)

print("Updated queries_test.go")
