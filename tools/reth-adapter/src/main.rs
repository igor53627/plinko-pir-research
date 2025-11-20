use clap::Parser;
use eyre::Result;
use reth_db::{
    cursor::DbCursorRO,
    database::Database,
    open_db_read_only,
    tables,
    transaction::DbTx,
};
use alloy_primitives::U256;
use std::fs::File;
use std::io::{BufWriter, Write};
use std::path::PathBuf;
use std::time::Instant;

#[derive(Parser, Debug)]
struct Args {
    #[arg(long)]
    db_path: PathBuf,
    #[arg(long, default_value = "output")]
    out_dir: PathBuf,
    #[arg(long)]
    limit: Option<usize>,
}

fn main() -> Result<()> {
    let args = Args::parse();

    println!("Opening Reth DB at {:?}...", args.db_path);

    // Use Default::default() for DatabaseArguments, relying on type inference
    let db = open_db_read_only(&args.db_path, Default::default())?;

    std::fs::create_dir_all(&args.out_dir)?;
    let db_file_path = args.out_dir.join("database.bin");
    let map_file_path = args.out_dir.join("address-mapping.bin");

    let mut db_writer = BufWriter::new(File::create(&db_file_path)?);
    let mut map_writer = BufWriter::new(File::create(&map_file_path)?);

    println!("Starting export...");
    let start_time = Instant::now();

    let tx = db.tx()?;
    let mut cursor = tx.cursor_read::<tables::PlainAccountState>()?;

    let mut count = 0;
    let walker = cursor.walk(None)?;
    for entry in walker {
        let (address, account) = entry?;
        
        map_writer.write_all(address.as_slice())?;

        let balance_u256: U256 = account.balance;
        let balance_bytes = balance_u256.to_be_bytes::<32>();
        db_writer.write_all(&balance_bytes)?;

        count += 1;
        if count % 1_000_000 == 0 {
            println!("Processed {} million accounts...", count / 1_000_000);
        }
        if let Some(lim) = args.limit {
            if count >= lim {
                break;
            }
        }
    }

    db_writer.flush()?;
    map_writer.flush()?;

    let elapsed = start_time.elapsed();
    println!("Done! Exported {} accounts in {:.2?}", count, elapsed);
    println!("Outputs:");
    println!("  Database: {:?}", db_file_path);
    println!("  Mapping:  {:?}", map_file_path);

    Ok(())
}
