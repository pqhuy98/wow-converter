import chalk from 'chalk';

import { Vector3 } from '@/lib/math/common';

import { WoWAttachmentID } from '../../animation/bones_mapper';
import { MDLModify } from '.';

// Only map WoW attachment points that have a valid WC3 equivalent.
// E.g. WoW ShoulderRight/ShoulderLeft do not exist in WC3, so we use Medium/Large as proxies.
const WoWToWC3AttachmentMap: Partial<Record<WoWAttachmentID, string>> = {
  [WoWAttachmentID.Head]: 'Head',
  [WoWAttachmentID.HandRight]: 'Hand Right',
  [WoWAttachmentID.HandLeft]: 'Hand Left',
  [WoWAttachmentID.ShoulderRight]: 'Medium',
  [WoWAttachmentID.ShoulderLeft]: 'Large',
  [WoWAttachmentID.LeftFoot]: 'Foot Left',
  [WoWAttachmentID.RightFoot]: 'Foot Right',
  [WoWAttachmentID.Chest]: 'Chest',
  [WoWAttachmentID.PlayerName]: 'Overhead',
  [WoWAttachmentID.Base]: 'Origin',
  // Add more as needed
};

export function addWc3AttachmentPoint(this: MDLModify) {
  // Only map WoW attachment points that have a valid WC3 equivalent.
  // E.g. WoW ArmL/ArmR do not exist in WC3, so we use Medium/Large as proxies.
  this.mdl.wowAttachments.forEach((wowAttachment) => {
    const bone = wowAttachment.bone;
    const wowAttachmentId = wowAttachment.wowAttachmentId as WoWAttachmentID;
    const wc3Key = WoWToWC3AttachmentMap[wowAttachmentId];
    const attachmentName = wc3Key
      ? `${wc3Key} Ref`
      : `Wow:${wowAttachmentId}:${Object.keys(WoWAttachmentID)[Object.values(WoWAttachmentID).indexOf(wowAttachmentId)]}`;

    this.mdl.attachments.push({
      attachmentId: 0,
      path: '',
      type: 'AttachmentPoint',
      name: attachmentName,
      parent: bone,
      pivotPoint: [...wowAttachment.bone.pivotPoint],
      flags: [],
    });
  });
  return this;
}

export function setWowAttachmentScale(this: MDLModify, wowAttachmentId: WoWAttachmentID, scale: number) {
  const attachment = this.mdl.wowAttachments.find((a) => a.wowAttachmentId === wowAttachmentId);
  if (!attachment) {
    console.error(chalk.red(`Cannot find wow attachment ${wowAttachmentId}`));
    return this;
  }
  attachment.bone.scaling = {
    interpolation: 'DontInterp',
    keyFrames: new Map(this.mdl.sequences.map((s) => <[number, Vector3]>[s.interval[0], [scale, scale, scale]])),
    type: 'scaling',
  };
  return this;
}
