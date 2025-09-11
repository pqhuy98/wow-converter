import { getWoWAttachmentName, WoWAttachmentID } from '@/lib/objmdl/animation/bones_mapper';

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
      : `Wow:${wowAttachmentId}:${getWoWAttachmentName(wowAttachmentId)}`;

    if (!this.mdl.globalSequences.length) {
      this.mdl.globalSequences.push({ id: -1, duration: 1000 });
    }

    this.mdl.attachments.push({
      attachmentId: 0,
      path: '',
      type: 'AttachmentPoint',
      name: attachmentName,
      parent: bone,
      pivotPoint: [...wowAttachment.bone.pivotPoint],
      data: {
        wowAttachment,
      },
      flags: [],
      scaling: {
        interpolation: 'DontInterp',
        globalSeq: this.mdl.globalSequences[0],
        keyFrames: new Map([[0, [1, 1, 1]]]),
        type: 'scaling',
      },
      translation: {
        interpolation: 'DontInterp',
        globalSeq: this.mdl.globalSequences[0],
        keyFrames: new Map([[0, [0, 0, 0]]]),
        type: 'translation',
      },
    });
  });
  return this;
}
