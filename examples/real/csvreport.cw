// csvreport: read a CSV file of sales and total it by region.
//
//     ccwig csvreport.cw data/sales.csv
//
// A report has to read three things it did not have before: a file it was
// named on the command line, fields taken apart out of the text, and numbers
// turned out of those text fields. Each of those can fail, and the way this
// program handles failure is the interesting part - a row it cannot read is
// counted and reported at the end, not dropped in silence and not allowed to
// stop the report.
//
// The language has no `continue`, so the checks nest instead of jumping back
// to the top of the loop. The shape is a fair statement of what the loop
// does: only a well-formed row reaches the totals.

// toInt reads a whole number out of a field, or null if the field is not one.
fn toInt(s) {
	if len(s) == 0 {
		return null;
	}

	let i = 0;
	let negative = false;

	if s[0] == "-" {
		negative = true;
		i = 1;
	}

	let n = 0;
	let digits = 0;

	while i < len(s) {
		let ch = s[i];

		if ch < "0" || ch > "9" {
			return null;
		}

		n = n * 10 + (ord(ch) - ord("0"));
		digits = digits + 1;
		i = i + 1;
	}

	if digits == 0 {
		return null;
	}
	if negative {
		return 0 - n;
	}
	return n;
}

fn padRight(s, width) {
	let out = s;
	while len(out) < width {
		out = out + " ";
	}
	return out;
}

fn padLeft(s, width) {
	let out = s;
	while len(out) < width {
		out = " " + out;
	}
	return out;
}

// indexOf is what a map would do, in a language that had one. The lists are
// short and the scan is a handful of comparisons, which is the honest trade
// at this size.
fn indexOf(names, name) {
	for let i = 0; i < len(names); i = i + 1 {
		if names[i] == name {
			return i;
		}
	}
	return -1;
}

let argv = args();
if len(argv) == 0 {
	print("usage: csvreport <file.csv>");
	exit(2);
}

let rows = split(readFile(argv[0]), "\n");

let names = [];
let units = [];
let revenue = [];
let skipped = 0;

for let i = 0; i < len(rows); i = i + 1 {
	let row = rows[i];

	// The first row is the header, and the newline at the end of the file
	// leaves an empty row after it.
	if i > 0 && len(row) > 0 {
		let fields = split(row, ",");

		if len(fields) != 3 {
			skipped = skipped + 1;
		} else {
			let u = toInt(fields[1]);
			let r = toInt(fields[2]);

			if u == null || r == null {
				skipped = skipped + 1;
			} else {
				let at = indexOf(names, fields[0]);

				if at < 0 {
					names = push(names, fields[0]);
					units = push(units, u);
					revenue = push(revenue, r);
				} else {
					units[at] = units[at] + u;
					revenue[at] = revenue[at] + r;
				}
			}
		}
	}
}

// The biggest region first, which is the order a report is read in. Three
// parallel lists, so every swap moves all three together or the rows stop
// describing the same region.
for let i = 0; i < len(names) - 1; i = i + 1 {
	for let j = 0; j < len(names) - 1 - i; j = j + 1 {
		if revenue[j] < revenue[j + 1] {
			let name = names[j];
			names[j] = names[j + 1];
			names[j + 1] = name;

			let u = units[j];
			units[j] = units[j + 1];
			units[j + 1] = u;

			let r = revenue[j];
			revenue[j] = revenue[j + 1];
			revenue[j + 1] = r;
		}
	}
}

let rule = "-----------  -------  ---------";

print(padRight("region", 11) + padLeft("units", 7) + padLeft("revenue", 11));
print(rule);

let totalUnits = 0;
let totalRevenue = 0;

for let i = 0; i < len(names); i = i + 1 {
	print(padRight(names[i], 11) + padLeft(str(units[i]), 7) + padLeft(str(revenue[i]), 11));
	totalUnits = totalUnits + units[i];
	totalRevenue = totalRevenue + revenue[i];
}

print(rule);
print(padRight("total", 11) + padLeft(str(totalUnits), 7) + padLeft(str(totalRevenue), 11));

// A row the report could not read is worth saying out loud. A file that is
// quietly missing a tenth of its rows is the kind of report nobody notices
// until the number is used.
if skipped > 0 {
	print("skipped " + str(skipped) + " unreadable row(s)");
}
