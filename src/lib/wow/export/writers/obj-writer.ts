/**
 * Wavefront OBJ writer, ported from wow.export (src/js/3D/writers/OBJWriter.js).
 * Builds the file content in memory and flushes via the output sink, producing
 * byte-identical output to wow.export's streaming FileWriter.
 */
import { constants } from '../../formats/constants';
import { outputFileExists, writeOutputFile } from './output-sink';

interface OBJMesh {
  name: string;
  triangles: number[];
  matName?: string;
}

export class OBJWriter {
  out: string;

  verts: number[] = [];

  normals: number[] = [];

  uvs: number[][] = [];

  meshes: OBJMesh[] = [];

  name = 'Mesh';

  mtl?: string;

  constructor(out: string) {
    this.out = out;
  }

  /** Set the name of the material library. */
  setMaterialLibrary(name: string): void {
    this.mtl = name;
  }

  /** Set the name of this model. */
  setName(name: string): void {
    this.name = name;
  }

  /** Set the vertex array for this writer. */
  setVertArray(verts: number[]): void {
    this.verts = verts;
  }

  /** Set the normals array for this writer. */
  setNormalArray(normals: number[]): void {
    this.normals = normals;
  }

  /** Add a UV array for this writer. */
  addUVArray(uv: number[]): void {
    this.uvs.push(uv);
  }

  /** Add a mesh to this writer. */
  addMesh(name: string, triangles: number[], matName?: string): void {
    this.meshes.push({ name, triangles, matName });
  }

  /** Produce the OBJ file content (identical to wow.export's line output). */
  getContent(): string {
    const lines: string[] = [];

    // Write header.
    lines.push(`# Exported using wow.export v${constants.VERSION}`);
    lines.push(`o ${this.name}`);

    // Link material library.
    if (this.mtl) lines.push(`mtllib ${this.mtl}`);

    const usedIndices = new Set<number>();
    this.meshes.forEach((mesh) => mesh.triangles.forEach((index) => usedIndices.add(index)));

    const vertMap = new Map<number, number>();
    const normalMap = new Map<number, number>();
    const uvMap = new Map<number, number>();

    // Write verts.
    const verts = this.verts;
    for (let i = 0, j = 0, u = 0, n = verts.length; i < n; j++, i += 3) {
      if (usedIndices.has(j)) {
        vertMap.set(j, u++);
        lines.push(`v ${verts[i]} ${verts[i + 1]} ${verts[i + 2]}`);
      }
    }

    // Write normals.
    const normals = this.normals;
    for (let i = 0, j = 0, u = 0, n = normals.length; i < n; j++, i += 3) {
      if (usedIndices.has(j)) {
        normalMap.set(j, u++);
        lines.push(`vn ${normals[i]} ${normals[i + 1]} ${normals[i + 2]}`);
      }
    }

    // Write UVs
    const layerCount = this.uvs.length;
    const hasUV = layerCount > 0;
    if (hasUV) {
      for (let uvIndex = 0; uvIndex < layerCount; uvIndex++) {
        const uv = this.uvs[uvIndex];

        let prefix = 'vt';

        // Use non-standard properties (vt2, vt3, etc) for additional UV layers.
        if (uvIndex > 0) prefix += (uvIndex + 1);

        for (let i = 0, j = 0, u = 0, n = uv.length; i < n; j++, i += 2) {
          if (usedIndices.has(j)) {
            // Build the index reference using just the first layer
            // since it will be identical for all other layers.
            if (uvIndex === 0) uvMap.set(j, u++);

            lines.push(`${prefix} ${uv[i]} ${uv[i + 1]}`);
          }
        }
      }
    }

    // Write meshes.
    for (const mesh of this.meshes) {
      lines.push(`g ${mesh.name}`);
      lines.push('s 1');

      if (mesh.matName) lines.push(`usemtl ${mesh.matName}`);

      const triangles = mesh.triangles;

      for (let i = 0, n = triangles.length; i < n; i += 3) {
        const pointA = `${vertMap.get(triangles[i])! + 1}/${hasUV ? uvMap.get(triangles[i])! + 1 : ''}/${normalMap.get(triangles[i])! + 1}`;
        const pointB = `${vertMap.get(triangles[i + 1])! + 1}/${hasUV ? uvMap.get(triangles[i + 1])! + 1 : ''}/${normalMap.get(triangles[i + 1])! + 1}`;
        const pointC = `${vertMap.get(triangles[i + 2])! + 1}/${hasUV ? uvMap.get(triangles[i + 2])! + 1 : ''}/${normalMap.get(triangles[i + 2])! + 1}`;

        lines.push(`f ${pointA} ${pointB} ${pointC}`);
      }
    }

    return `${lines.join('\n')}\n`;
  }

  /** Write the OBJ file. */
  async write(overwrite = true): Promise<void> {
    // If overwriting is disabled, check file existence.
    if (!overwrite && await outputFileExists(this.out)) return;

    await writeOutputFile(this.out, this.getContent());
  }
}

export default OBJWriter;
