-- Migration: Replace LDAP-imported employees with portal users
--
-- This migration:
-- 1. Adds user_id (uint32) column to asset_assets and asset_assignments
-- 2. Maps existing employee assignments to portal users by matching email
-- 3. Removes old employee_id/employee_name columns
-- 4. Drops the asset_employees table
--
-- Prerequisites:
--   - sys_users table must exist with id, email columns
--   - Run this BEFORE deploying the new code

-- Step 1: Add new columns
ALTER TABLE asset_assets ADD COLUMN IF NOT EXISTS user_id INTEGER;
ALTER TABLE asset_assignments ADD COLUMN IF NOT EXISTS user_id INTEGER NOT NULL DEFAULT 0;
ALTER TABLE asset_assignments ADD COLUMN IF NOT EXISTS user_name VARCHAR DEFAULT '';

-- Step 2: Map asset_assets.employee_id -> user_id via email matching
-- asset_employees.email <-> sys_users.email
UPDATE asset_assets a
SET user_id = su.id
FROM asset_employees ae
JOIN sys_users su ON LOWER(ae.email) = LOWER(su.email)
WHERE a.employee_id = ae.id
  AND a.employee_id IS NOT NULL
  AND a.employee_id != '';

-- Step 3: Map asset_assignments.employee_id -> user_id and user_name via email matching
UPDATE asset_assignments aa
SET user_id = su.id,
    user_name = COALESCE(su.realname, su.username, '')
FROM asset_employees ae
JOIN sys_users su ON LOWER(ae.email) = LOWER(su.email)
WHERE aa.employee_id = ae.id
  AND aa.employee_id IS NOT NULL
  AND aa.employee_id != '';

-- Step 4: For assignments that couldn't be mapped, set user_name from old employee data
UPDATE asset_assignments aa
SET user_name = CONCAT(ae.first_name, ' ', ae.last_name)
FROM asset_employees ae
WHERE aa.employee_id = ae.id
  AND aa.user_id = 0
  AND aa.employee_id IS NOT NULL
  AND aa.employee_id != '';

-- Step 5: Drop old columns
ALTER TABLE asset_assets DROP COLUMN IF EXISTS employee_id;
ALTER TABLE asset_assignments DROP COLUMN IF EXISTS employee_id;
ALTER TABLE asset_assignments DROP COLUMN IF EXISTS employee_name;

-- Step 6: Add indexes
CREATE INDEX IF NOT EXISTS idx_asset_assets_user_id ON asset_assets(user_id);
CREATE INDEX IF NOT EXISTS idx_asset_assignments_user_id ON asset_assignments(user_id);

-- Step 7: Drop the employee table (optional - keep if you need historical reference)
-- DROP TABLE IF EXISTS asset_employees;

-- Verify: Check for unmapped assignments (user_id = 0 means no portal user match found)
-- SELECT COUNT(*) AS unmapped_assignments FROM asset_assignments WHERE user_id = 0;
-- SELECT COUNT(*) AS unmapped_assets FROM asset_assets WHERE user_id IS NULL AND status = 2;
