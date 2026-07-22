import { describe, it, expect } from 'vitest';
import { portCoords, bezierPath } from './mappingLines';

const container = { left: 100, right: 900, top: 50, height: 600 };

describe('portCoords', () => {
  it('maps JIRA right-edge and Fern left-edge to container-relative points', () => {
    const jira = { left: 600, right: 800, top: 100, height: 40 };
    const fern = { left: 150, right: 350, top: 300, height: 40 };
    expect(portCoords(jira, fern, container)).toEqual({
      x1: 700, // 800 - 100
      y1: 70, //  100 + 20 - 50
      x2: 50, //  150 - 100
      y2: 270, // 300 + 20 - 50
    });
  });
});

describe('bezierPath', () => {
  it('produces a cubic path between the two endpoints', () => {
    const d = bezierPath({ x1: 700, y1: 70, x2: 50, y2: 270 });
    expect(d.startsWith('M 700 70 C')).toBe(true);
    expect(d).toContain('50 270');
  });
  it('uses a minimum horizontal pull even when endpoints are close', () => {
    const d = bezierPath({ x1: 100, y1: 10, x2: 110, y2: 10 });
    // dx clamps to 24 → first control x is 100 - 24 = 76
    expect(d).toContain('76 10');
  });
});
