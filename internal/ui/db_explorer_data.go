package ui

// defaultExplorerRoots returns the initial hardcoded tree shown in the DB explorer.
// Replace this function with a real data source once live DB introspection is wired up.
func defaultExplorerRoots() []*explorerNode {
	devices := &explorerNode{
		kind: nodeKindTable, label: "devices", meta: "21", expanded: true,
		children: []*explorerNode{
			{kind: nodeKindCol, label: "udid", meta: "TEXT", isPK: true, disabled: true},
			{kind: nodeKindCol, label: "name", meta: "TEXT", disabled: true},
			{kind: nodeKindCol, label: "os", meta: "TEXT", disabled: true},
			{kind: nodeKindCol, label: "state", meta: "TEXT", disabled: true},
			{kind: nodeKindCol, label: "family", meta: "TEXT", disabled: true},
			{kind: nodeKindCol, label: "booted_at", meta: "DATETIME", disabled: true},
			{kind: nodeKindCol, label: "cpu_pct", meta: "REAL", disabled: true},
			{kind: nodeKindCol, label: "… 4 more", disabled: true},
		},
	}

	simctl := &explorerNode{
		kind: nodeKindDB, label: "simctl.db", meta: "SQLite",
		connColor: ColorOrange, expanded: true,
		children: []*explorerNode{
			{
				kind: nodeKindFolder, label: "Tables", meta: "8", expanded: true,
				children: []*explorerNode{
					{kind: nodeKindTable, label: "apps", meta: "142"},
					devices,
					{kind: nodeKindTable, label: "device_runtimes", meta: "38"},
					{kind: nodeKindTable, label: "install_logs", meta: "1.2k"},
					{kind: nodeKindTable, label: "processes", meta: "847"},
					{kind: nodeKindTable, label: "preferences", meta: "93"},
					{kind: nodeKindTable, label: "media_assets", meta: "412"},
					{kind: nodeKindTable, label: "screenshots", meta: "28"},
				},
			},
			{kind: nodeKindFolder, label: "Views", meta: "3"},
			{kind: nodeKindFolder, label: "Indexes", meta: "14"},
			{kind: nodeKindFolder, label: "Triggers", meta: "2"},
			{kind: nodeKindFolder, label: "Sequences", meta: "n/a", disabled: true},
		},
	}

	return []*explorerNode{
		simctl,
		{kind: nodeKindDB, label: "logs.db", meta: "SQLite", connColor: ColorInfo},
		{kind: nodeKindDB, label: "analytics.duckdb", meta: "offline", disabled: true, connColor: ColorFgFaint},
	}
}
