# Memory Tool Example

This example demonstrates how to use the memory tool to store, retrieve, update, and delete memories with semantic search capabilities.

## Prerequisites

1. Go 1.23.6 or later
2. A PostgreSQL database with the pgvector extension

## Setup

Set the DATABASE_URL environment variable to your PostgreSQL connection string:
```bash
export DATABASE_URL="your-postgresql-connection-string"
```

## Running the Example

1. Navigate to the example directory:
   ```bash
   cd examples/memory
   ```

2. (Optional) Test your database connection:
   ```bash
   ./test_db.sh
   ```

3. Run the example:
   ```bash
   go run main.go
   ```

## Database Connection Options

The example supports different database configurations:

1. **Local Development (with Docker)**:
   ```bash
   export DATABASE_URL="postgresql://postgres:your-super-secret-and-long-postgres-password@localhost:5432/genai_memory?sslmode=disable"
   ```

2. **Remote PostgreSQL (with SSL)**:
   ```bash
   export DATABASE_URL="postgresql://username:password@hostname:port/database_name"
   ```

3. **Supabase**:
   ```bash
   export DATABASE_URL="postgresql://postgres:[YOUR-PASSWORD]@db.[YOUR-SUPABASE-PROJECT-ID].supabase.co:5432/postgres"
   ```

## Expected Output

The example will:
1. Store a memory with user preferences
2. Retrieve memories based on a semantic query
3. Update the stored memory
4. Delete the memory
5. Demonstrate the single operation tool

## Troubleshooting

If you encounter issues:

1. **Test your database connection separately**:
   ```bash
   ./test_db.sh
   ```

2. **For "Tenant or user not found" errors with Supabase**:
   - Verify your connection string is correct
   - Make sure you've created a Supabase project
   - Check that your credentials are valid

3. **For SSL connection issues**:
   - Try adding `?sslmode=disable` to your connection string for local development:
   ```bash
   export DATABASE_URL="your-connection-string?sslmode=disable"
   ```

4. **Verify the DATABASE_URL environment variable is set correctly**:
   ```bash
   echo $DATABASE_URL
   ```