# Vendored species value + genus display name data

Copies of `edexotracker/vendor/bioscan_bio_data/value_table.json` (from EDMC-BioScan, GPLv2)
and `edexotracker/vendor/explodata_bio_data/genus_names.json` (from EDMC-ExploData, GPLv2) --
see those directories' own READMEs for full provenance/refresh instructions. Copied here
(rather than importing the `edexotracker` package at runtime) so `standalone/` has zero import
dependency on the rest of this repo and can be distributed/run entirely on its own.

To refresh after a game update: re-run the build scripts in the source vendor directories, then
copy the two output JSON files here again.
