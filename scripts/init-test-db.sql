-- Runs once, when the Postgres volume is first created.
-- Tests use their own database so they never touch your dev data.
CREATE DATABASE wallet_test;
