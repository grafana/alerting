package kinds

config: {
	codegen: {
		goGenPath: "./pkg/apis"
	}
	definitions: {
		// Emit the app manifest as JSON to definitions/<appName>-manifest.json.
		genManifest:     true
		genCRDs:         false
		manifestSchemas: true
		manifestVersion: "v1alpha2"
		path:            "definitions"
		encoding:        "json"
	}
	kinds: {
		grouping: "group"
	}
}
