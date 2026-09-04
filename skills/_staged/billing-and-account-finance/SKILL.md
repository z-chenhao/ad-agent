---
name: billing-and-account-finance
description: Monitoring account balance, budget, transaction, and billing readiness without initiating transfers or changing payment authority.
---

# Billing and account finance

This workflow remains staged until typed advertiser balance, account budget, transaction,
invoice, billing-group, and funding-mode reads exist.

Bind every record to the advertiser/business center and currency. Distinguish cash
balance, credit line, account budget cap, daily delivery budget, pending charge, invoice,
tax, refund, transfer, and coupon; they are not interchangeable. Preserve transaction
time, settlement state, reference, and sign. Never infer available spend from a campaign
budget.

Estimate runway only from a stated balance field and a clearly labeled spend-rate
window: `runway_days = usable_balance / average_daily_spend`. Mark it unavailable for
mixed currencies, partial spend windows, unsettled balance, or unknown funding mode.
Projection is not a payment guarantee.

Return current balance/budget facts, recent material transactions, reconciliation gaps,
runway assumptions, and required owner action. Minimize sensitive finance data in model
context and presentation. Transfers, payment methods, billing-group membership, credit
authority, invoices, refunds, and ownership changes remain outside agent authority until
separately designed and approved.
