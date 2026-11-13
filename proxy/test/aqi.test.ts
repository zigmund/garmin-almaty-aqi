import { expect, test } from "vitest";
import { aqiCategory, pmToAqi } from "../src/aqi";

test.each([[0,0],[8.8,49],[9.0,50],[9.1,51],[35.4,100],[35.5,101],[55.4,150],[125.4,200],[225.4,300],[325.4,500],[999,500],[8.86,49]])
  ('pmToAqi(%f) = %i', (pm, aqi) => expect(pmToAqi(pm)).toBe(aqi));

test('categories', () => {
  expect([0,50,51,100,101,150,151,200,201,300,301,500].map(aqiCategory)).toEqual([0,0,1,1,2,2,3,3,4,4,5,5]);
});

test('negative pm clamps to 0', () => expect(pmToAqi(-3)).toBe(0));
