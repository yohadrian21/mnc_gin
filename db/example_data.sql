-- Insert sample data into the user table
INSERT INTO "user" (id, phone_number, pin, first_name, last_name, address, balance, created_at, updated_at)
VALUES
    (uuid_generate_v4(), '1234567890', '1234', 'John', 'Doe', '123 Main St, Cityville', 1000.00, NOW(), NOW()),
    (uuid_generate_v4(), '0987654321', '5678', 'Jane', 'Smith', '456 Elm St, Townsville', 2500.50, NOW(), NOW()),
    (uuid_generate_v4(), '5555555555', '9101', 'Alice', 'Johnson', '789 Oak St, Villageton', 150.75, NOW(), NOW());

-- Insert sample data into the transactions table using 'debit' and 'credit' for the type
INSERT INTO transactions (id, user_id, "type", amount, balance_before, balance_after, remarks, status, created_at, payment_id, top_up_id, transfer_id)
VALUES
    -- Credit Transactions (Deposits)
    (uuid_generate_v4(), (SELECT id FROM "user" WHERE first_name = 'John' AND last_name = 'Doe'), 'credit', 200.00, 1000.00, 1200.00, 'Initial deposit', 'completed', NOW(), NULL, uuid_generate_v4(), NULL),
    
    -- Debit Transactions (Withdrawals)
    (uuid_generate_v4(), (SELECT id FROM "user" WHERE first_name = 'John' AND last_name = 'Doe'), 'debit', 100.00, 1200.00, 1100.00, 'ATM withdrawal', 'completed', NOW(), NULL, NULL, NULL),

    -- Credit Transaction for Jane Smith
    (uuid_generate_v4(), (SELECT id FROM "user" WHERE first_name = 'Jane' AND last_name = 'Smith'), 'credit', 500.00, 2500.50, 3000.50, 'Online transfer', 'completed', NOW(), NULL, uuid_generate_v4(), NULL),
    
    -- Debit Transaction for Jane Smith
    (uuid_generate_v4(), (SELECT id FROM "user" WHERE first_name = 'Jane' AND last_name = 'Smith'), 'debit', 250.00, 3000.50, 2750.50, 'Transfer to friend', 'completed', NOW(), NULL, NULL, uuid_generate_v4()),

    -- Credit Transaction for Alice Johnson
    (uuid_generate_v4(), (SELECT id FROM "user" WHERE first_name = 'Alice' AND last_name = 'Johnson'), 'credit', 50.00, 150.75, 200.75, 'Mobile app deposit', 'completed', NOW(), NULL, uuid_generate_v4(), NULL),
    
    -- Debit Transaction for Alice Johnson
    (uuid_generate_v4(), (SELECT id FROM "user" WHERE first_name = 'Alice' AND last_name = 'Johnson'), 'debit', 20.00, 200.75, 180.75, 'Bill payment', 'completed', NOW(), NULL, NULL, NULL);
