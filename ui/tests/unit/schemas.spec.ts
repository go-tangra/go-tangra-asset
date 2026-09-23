import { describe, expect, it } from 'vitest'
import { assetSchema, categorySchema, supplierSchema, locationSchema, consumableSchema, licenseSchema, insurancePolicySchema, policyAssetSchema, backupImportSchema } from '@/schemas'

// Outputs must equal the OpenAPI write payloads (api/openapi/asset.yaml): blank
// optionals are dropped, numbers are numbers, dates are ISO 8601 UTC.
describe('asset schemas', () => {
  it('asset: required name, normalised numbers/dates, blanks dropped, tag map limited', () => {
    expect(assetSchema.safeParse({}).success).toBe(false)
    const r = assetSchema.safeParse({ name: '  Laptop ', asset_tag: '', purchase_date: '2026-01-05', purchase_cost: '12.5', warranty_months: '', depreciation_rate: '', status: '', tags: { env: 'lab' } })
    expect(r.success).toBe(true)
    expect(r.data).toEqual({ name: 'Laptop', purchase_date: '2026-01-05T00:00:00.000Z', purchase_cost: 12.5, warranty_months: 0, useful_life_years: 0, salvage_value: 0, depreciation_rate: 0, tags: { env: 'lab' } })
    expect(assetSchema.safeParse({ name: 'x', depreciation_rate: 1.5 }).success).toBe(false)
    expect(assetSchema.safeParse({ name: 'x', purchase_cost: -1 }).success).toBe(false)
    expect(assetSchema.safeParse({ name: 'x', purchase_date: 'not a date' }).success).toBe(false)
    expect(assetSchema.safeParse({ name: 'x', useful_life_years: 101 }).success).toBe(false)
    expect(assetSchema.safeParse({ name: 'x', status: 'assigned' }).success).toBe(false) // assigned only through Assign
    const many = Object.fromEntries(Array.from({ length: 65 }, (_, i) => ['k' + i, 'v']))
    expect(assetSchema.safeParse({ name: 'x', tags: many }).success).toBe(false)
    expect(assetSchema.safeParse({ name: 'x'.repeat(201) }).success).toBe(false)
  })
  it('category / supplier / location', () => {
    expect(categorySchema.safeParse({ name: 'Laptops', icon: 'mdi-laptop', parent_id: '' }).data).toEqual({ name: 'Laptops', icon: 'mdi-laptop', tags: {} })
    expect(categorySchema.safeParse({ name: 'Laptops', icon: 'laptop' }).success).toBe(false)
    expect(supplierSchema.safeParse({ name: 'ACME', email: ' Sales@Acme.TEST ', website: 'https://acme.test', status: 'active' }).data).toMatchObject({ email: 'sales@acme.test', website: 'https://acme.test', status: 'active' })
    expect(supplierSchema.safeParse({ name: 'ACME', email: 'nope' }).success).toBe(false)
    expect(supplierSchema.safeParse({ name: 'ACME', website: 'acme' }).success).toBe(false)
    expect(supplierSchema.safeParse({ name: 'ACME', status: 'gone' }).success).toBe(false)
    expect(locationSchema.safeParse({ name: 'HQ', status: 'planned', email: '' }).data).toEqual({ name: 'HQ', status: 'planned' })
    expect(locationSchema.safeParse({ name: '' }).success).toBe(false)
  })
  it('consumable / license / insurance: ranges and valid_to ≥ valid_from', () => {
    expect(consumableSchema.safeParse({ name: 'Toner', amount: '3', min_amount: 1 }).data).toMatchObject({ amount: 3, min_amount: 1, purchase_cost: 0 })
    expect(consumableSchema.safeParse({ name: 'Toner', amount: -1 }).success).toBe(false)
    expect(consumableSchema.safeParse({ name: 'Toner', amount: 1.5 }).success).toBe(false)
    const ok = licenseSchema.safeParse({ name: 'Office', valid_from: '2026-01-01', valid_to: '2026-12-31', status: 'active' })
    expect(ok.success).toBe(true)
    const bad = licenseSchema.safeParse({ name: 'Office', valid_from: '2026-12-31', valid_to: '2026-01-01' })
    expect(bad.success).toBe(false)
    expect(bad.success ? [] : bad.error.issues.map((i) => i.path.join('.'))).toEqual(['valid_to'])
    expect(insurancePolicySchema.safeParse({ name: 'P', policy_number: '' }).success).toBe(false)
    expect(insurancePolicySchema.safeParse({ name: 'P', policy_number: 'N1', coverage_type: 'cyber', premium_amount: '99.90', valid_from: '2026-01-01', valid_to: '2025-01-01' }).success).toBe(false)
    expect(insurancePolicySchema.safeParse({ name: 'P', policy_number: 'N1', coverage_type: 'cyber', premium_amount: '99.90' }).data).toMatchObject({ coverage_type: 'cyber', premium_amount: 99.9, deductible: 0 })
    expect(policyAssetSchema.safeParse({ asset_id: '', covered_value: 1 }).success).toBe(false)
    expect(policyAssetSchema.safeParse({ asset_id: 'a1', covered_value: '' }).data).toEqual({ asset_id: 'a1', covered_value: 0 })
  })
  it('backup import: needs a file under 50 MiB and a mode', () => {
    expect(backupImportSchema.safeParse({ mode: 'skip' }).success).toBe(false)
    expect(backupImportSchema.safeParse({ file: new File(['{}'], 'b.json'), mode: 'merge' }).success).toBe(false)
    expect(backupImportSchema.safeParse({ file: new File(['{}'], 'b.json'), mode: 'overwrite' }).success).toBe(true)
  })
  it('fuzz: random inputs never throw and outputs are normalised', () => {
    const rnd = () => Math.random().toString(36).slice(2, 2 + Math.floor(Math.random() * 12))
    for (let i = 0; i < 300; i++) {
      const input: Record<string, unknown> = { name: rnd(), serial: rnd(), purchase_cost: Math.random() < 0.5 ? rnd() : Math.random() * 1000, purchase_date: Math.random() < 0.5 ? rnd() : '2026-02-03', tags: Math.random() < 0.3 ? null : { [rnd() || 'k']: rnd() } }
      const r = assetSchema.safeParse(input)
      if (r.success) {
        expect(r.data.name).toBe(r.data.name.trim())
        expect(typeof r.data.purchase_cost).toBe('number')
        if (r.data.purchase_date) expect(r.data.purchase_date).toMatch(/Z$/)
      }
    }
  })
})
