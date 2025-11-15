const DATABASE_BYTES = 44_606_944;       // database.bin (8 bytes × 5,575,868 entries)
const ADDRESS_MAPPING_BYTES = 133_820_832; // address-mapping.bin (24 bytes × 5,575,868 entries)
const HINT_BYTES = 87_818_272;           // hint.bin derived locally (~80 MB)
const ADDRESS_COUNT = 5_575_868;

const bytesToMB = (bytes) => Number((bytes / 1024 / 1024).toFixed(1));

export const DATASET_STATS = {
  addressCount: ADDRESS_COUNT,
  databaseBytes: DATABASE_BYTES,
  addressMappingBytes: ADDRESS_MAPPING_BYTES,
  hintBytes: HINT_BYTES,
  totalSnapshotBytes: DATABASE_BYTES + ADDRESS_MAPPING_BYTES,
};

export const DATASET_DISPLAY = {
  databaseMB: bytesToMB(DATABASE_BYTES),
  addressMappingMB: bytesToMB(ADDRESS_MAPPING_BYTES),
  hintMB: bytesToMB(HINT_BYTES),
  totalSnapshotMB: bytesToMB(DATABASE_BYTES + ADDRESS_MAPPING_BYTES),
};
