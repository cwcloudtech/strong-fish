/// The API's response shapes, decoded into plain Dart objects.
///
/// Every model parses defensively: a field the server hasn't sent (an older
/// deployment, or a payload that legitimately omits it) becomes a sensible
/// default rather than a runtime type error, since a single bad cast takes the
/// whole screen down.
library;

double _toDouble(dynamic value, [double fallback = 0]) {
  if (value is num) return value.toDouble();
  if (value is String) return double.tryParse(value) ?? fallback;
  return fallback;
}

double? _toDoubleOrNull(dynamic value) {
  if (value == null) return null;
  if (value is num) return value.toDouble();
  if (value is String) return double.tryParse(value);
  return null;
}

int _toInt(dynamic value, [int fallback = 0]) {
  if (value is num) return value.toInt();
  if (value is String) return int.tryParse(value) ?? fallback;
  return fallback;
}

int? _toIntOrNull(dynamic value) {
  if (value == null) return null;
  if (value is num) return value.toInt();
  if (value is String) return int.tryParse(value);
  return null;
}

String _toString(dynamic value, [String fallback = '']) => value?.toString() ?? fallback;

Map<String, String> _toLabels(dynamic value) {
  if (value is Map) {
    return value.map((key, label) => MapEntry(key.toString(), label?.toString() ?? ''));
  }
  return const {};
}

DateTime _toDate(dynamic value) => DateTime.tryParse(_toString(value))?.toUtc() ?? DateTime.utc(1970);

/// A date that may legitimately be absent - an event with no stated end -
/// rather than _toDate's epoch fallback, which would render as 1970.
DateTime? _optionalDate(dynamic value) {
  final parsed = DateTime.tryParse(_toString(value));
  return parsed?.toLocal();
}

List<String> _toStringList(dynamic value) {
  if (value is List) return value.map((item) => item.toString()).toList();
  return const [];
}

/// A user as the app knows them: the connected account, or a post's author.
class User {
  final String id;
  final String email;
  final String name;
  final String surname;
  final String handle;
  /// The name the member picked. When [anonymous] is set it is the only name
  /// anybody else sees, and the handle is derived from it.
  final String username;
  final bool anonymous;
  final String bio;
  final String role;
  final String picture;
  final String locale;
  /// Who may read this profile: 'public', 'clubs' or 'private'. An unknown
  /// value reads as the narrowest, matching the API.
  final String profileVisibility;
  final String birthdate;
  final double bodyweight;
  final bool mfaEnabled;
  final String i18nCode;

  const User({
    required this.id,
    this.email = '',
    this.name = '',
    this.surname = '',
    this.handle = '',
    this.username = '',
    this.anonymous = false,
    this.bio = '',
    this.role = '',
    this.picture = '',
    this.locale = '',
    this.profileVisibility = 'private',
    this.birthdate = '',
    this.bodyweight = 0,
    this.mfaEnabled = false,
    this.i18nCode = '',
  });

  factory User.fromJson(Map<String, dynamic> json) => User(
        id: _toString(json['id']),
        email: _toString(json['email']),
        name: _toString(json['name']),
        surname: _toString(json['surname']),
        handle: _toString(json['handle']),
        username: _toString(json['username']),
        anonymous: json['anonymous'] == true,
        bio: _toString(json['bio']),
        role: _toString(json['role']),
        picture: _toString(json['picture']),
        locale: _toString(json['locale']),
        profileVisibility: _toString(json['profileVisibility'], 'private'),
        birthdate: _toString(json['birthdate']),
        bodyweight: _toDouble(json['bodyweight']),
        mfaEnabled: json['mfaEnabled'] == true,
        i18nCode: _toString(json['i18nCode']),
      );

  String get fullName => '$name $surname'.trim();

  /// Whether the account may create clubs, upload programs and extend the
  /// exercise catalog.
  bool get isCoach => role == 'coach' || role == 'superadmin';
  bool get isSuperadmin => role == 'superadmin';

  /// Whether the account may do anything beyond reading its own status.
  bool get isActive => role != 'disabled' && role != 'ban';
}

/// What a login returned: either a session, or a second-factor challenge.
class LoginResult {
  final String token;
  final bool mfaRequired;
  final String challengeToken;
  final bool hasTotp;
  final bool hasWebAuthn;

  const LoginResult({
    this.token = '',
    this.mfaRequired = false,
    this.challengeToken = '',
    this.hasTotp = false,
    this.hasWebAuthn = false,
  });

  factory LoginResult.fromJson(Map<String, dynamic> json) => LoginResult(
        token: _toString(json['token']),
        mfaRequired: json['mfaRequired'] == true,
        challengeToken: _toString(json['challengeToken']),
        hasTotp: json['hasTotp'] == true,
        hasWebAuthn: json['hasWebAuthn'] == true,
      );
}

class Club {
  final String id;
  final String name;
  final String description;
  final String city;
  final String role;
  final int memberCount;

  const Club({
    required this.id,
    this.name = '',
    this.description = '',
    this.city = '',
    this.role = '',
    this.memberCount = 0,
  });

  factory Club.fromJson(Map<String, dynamic> json) => Club(
        id: _toString(json['id']),
        name: _toString(json['name']),
        description: _toString(json['description']),
        city: _toString(json['city']),
        role: _toString(json['role']),
        memberCount: _toInt(json['memberCount']),
      );

  bool get canManage => role == 'owner' || role == 'admin';
}

class Exercise {
  final String id;
  final String slug;
  final Map<String, String> labels;
  final String category;
  final String oneRmRef;
  final bool bodyweight;
  final bool main;

  const Exercise({
    required this.id,
    this.slug = '',
    this.labels = const {},
    this.category = '',
    this.oneRmRef = '',
    this.bodyweight = false,
    this.main = false,
  });

  factory Exercise.fromJson(Map<String, dynamic> json) => Exercise(
        id: _toString(json['id']),
        slug: _toString(json['slug']),
        labels: _toLabels(json['labels']),
        category: _toString(json['category']),
        oneRmRef: _toString(json['oneRmRef']),
        bodyweight: json['bodyweight'] == true,
        main: json['main'] == true,
      );

  String label(String locale) => labels[locale]?.isNotEmpty == true
      ? labels[locale]!
      : (labels['en']?.isNotEmpty == true ? labels['en']! : slug);
}

/// One of the member's maxes. Everything the training screens show is computed
/// from these server-side, so changing one recalculates the whole program.
class OneRm {
  final String exerciseId;
  final String slug;
  final Map<String, String> labels;
  final double value;
  final DateTime updatedAt;

  OneRm({
    required this.exerciseId,
    this.slug = '',
    this.labels = const {},
    this.value = 0,
    DateTime? updatedAt,
  }) : updatedAt = updatedAt ?? DateTime.utc(1970);

  factory OneRm.fromJson(Map<String, dynamic> json) => OneRm(
        exerciseId: _toString(json['exerciseId']),
        slug: _toString(json['slug']),
        labels: _toLabels(json['labels']),
        value: _toDouble(json['value']),
        updatedAt: _toDate(json['updatedAt']),
      );

  String label(String locale) => labels[locale]?.isNotEmpty == true
      ? labels[locale]!
      : (labels['en']?.isNotEmpty == true ? labels['en']! : slug);
}

/// What the member logged for one prescribed set.
class SetLog {
  final int? actualReps;
  final double? actualRpe;
  final double? actualLoad;
  final String comment;
  final bool done;
  final double e1rm;

  const SetLog({
    this.actualReps,
    this.actualRpe,
    this.actualLoad,
    this.comment = '',
    this.done = false,
    this.e1rm = 0,
  });

  factory SetLog.fromJson(Map<String, dynamic> json) => SetLog(
        actualReps: _toIntOrNull(json['actualReps']),
        actualRpe: _toDoubleOrNull(json['actualRpe']),
        actualLoad: _toDoubleOrNull(json['actualLoad']),
        comment: _toString(json['comment']),
        done: json['done'] == true,
        e1rm: _toDouble(json['e1rm']),
      );
}

/// One prescribed set, with the load already resolved against this member's own
/// 1RM. [loadKnown] is false when the max it would come from hasn't been
/// recorded yet - the UI shows the spreadsheet's "?" for those.
class ProgramSet {
  final String id;
  final String exerciseId;
  final String exerciseSlug;
  final Map<String, String> exerciseLabels;
  final String exerciseOneRmRef;
  final bool bodyweight;
  final int reps;
  final double? rpe;
  final double? percentage;
  final double? absoluteLoad;
  final String loadMode;
  final double load;
  final double roundedLoad;
  final double computedPercentage;
  final bool loadKnown;
  final double oneRm;
  final String notes;
  final SetLog? log;

  const ProgramSet({
    required this.id,
    this.exerciseId = '',
    this.exerciseSlug = '',
    this.exerciseLabels = const {},
    this.exerciseOneRmRef = '',
    this.bodyweight = false,
    this.reps = 0,
    this.rpe,
    this.percentage,
    this.absoluteLoad,
    this.loadMode = '',
    this.load = 0,
    this.roundedLoad = 0,
    this.computedPercentage = 0,
    this.loadKnown = false,
    this.oneRm = 0,
    this.notes = '',
    this.log,
  });

  factory ProgramSet.fromJson(Map<String, dynamic> json) => ProgramSet(
        id: _toString(json['id']),
        exerciseId: _toString(json['exerciseId']),
        exerciseSlug: _toString(json['exerciseSlug']),
        exerciseLabels: _toLabels(json['exerciseLabels']),
        exerciseOneRmRef: _toString(json['exerciseOneRmRef']),
        bodyweight: json['bodyweight'] == true,
        reps: _toInt(json['reps']),
        rpe: _toDoubleOrNull(json['rpe']),
        percentage: _toDoubleOrNull(json['percentage']),
        absoluteLoad: _toDoubleOrNull(json['absoluteLoad']),
        loadMode: _toString(json['loadMode']),
        load: _toDouble(json['load']),
        roundedLoad: _toDouble(json['roundedLoad']),
        computedPercentage: _toDouble(json['computedPercentage']),
        loadKnown: json['loadKnown'] == true,
        oneRm: _toDouble(json['oneRm']),
        notes: _toString(json['notes']),
        log: json['log'] is Map<String, dynamic> ? SetLog.fromJson(json['log']) : null,
      );

  String label(String locale) => exerciseLabels[locale]?.isNotEmpty == true
      ? exerciseLabels[locale]!
      : (exerciseLabels['en']?.isNotEmpty == true ? exerciseLabels['en']! : exerciseSlug);

  bool get isBodyweight => loadMode == 'bodyweight';
}

/// A program as its coach sees it in the club's list.
class Program {
  final String id;
  final String clubId;
  final String name;
  final String description;
  final String authorName;
  final int weeks;
  final int dayCount;
  final int setCount;

  const Program({
    required this.id,
    this.clubId = '',
    this.name = '',
    this.description = '',
    this.authorName = '',
    this.weeks = 0,
    this.dayCount = 0,
    this.setCount = 0,
  });

  factory Program.fromJson(Map<String, dynamic> json) => Program(
        id: _toString(json['id']),
        clubId: _toString(json['clubId']),
        name: _toString(json['name']),
        description: _toString(json['description']),
        authorName: _toString(json['authorName']),
        weeks: _toInt(json['weeks']),
        dayCount: _toInt(json['dayCount']),
        setCount: _toInt(json['setCount']),
      );
}

/// A program with its sessions, as returned to a coach authoring it. The loads
/// on each set are resolved against the *coach's* own maxes here, which is what
/// makes the numbers concrete while writing.
class ProgramDetail {
  final Program program;
  final List<ProgramDay> days;

  const ProgramDetail({required this.program, this.days = const []});

  factory ProgramDetail.fromJson(Map<String, dynamic> json) => ProgramDetail(
        program: Program.fromJson(json),
        days: (json['days'] as List? ?? const [])
            .whereType<Map<String, dynamic>>()
            .map(ProgramDay.fromJson)
            .toList(),
      );
}

class ProgramDay {
  final String id;
  final int week;
  final int day;
  final String title;
  final List<ProgramSet> sets;

  const ProgramDay({
    required this.id,
    this.week = 0,
    this.day = 0,
    this.title = '',
    this.sets = const [],
  });

  factory ProgramDay.fromJson(Map<String, dynamic> json) => ProgramDay(
        id: _toString(json['id']),
        week: _toInt(json['week']),
        day: _toInt(json['day']),
        title: _toString(json['title']),
        sets: (json['sets'] as List? ?? const [])
            .whereType<Map<String, dynamic>>()
            .map(ProgramSet.fromJson)
            .toList(),
      );

  int get doneCount => sets.where((set) => set.log?.done == true).length;
}

/// A program assigned to the member, as listed on the training screen.
class Assignment {
  final String id;
  final String programId;
  final String programName;
  final String clubName;
  final String status;
  final String note;
  final String startDate;
  final int completedSets;
  final int totalSets;

  const Assignment({
    required this.id,
    this.programId = '',
    this.programName = '',
    this.clubName = '',
    this.status = 'active',
    this.note = '',
    this.startDate = '',
    this.completedSets = 0,
    this.totalSets = 0,
  });

  factory Assignment.fromJson(Map<String, dynamic> json) => Assignment(
        id: _toString(json['id']),
        programId: _toString(json['programId']),
        programName: _toString(json['programName']),
        clubName: _toString(json['clubName']),
        status: _toString(json['status'], 'active'),
        note: _toString(json['note']),
        startDate: _toString(json['startDate']),
        completedSets: _toInt(json['completedSets']),
        totalSets: _toInt(json['totalSets']),
      );
}

/// One assigned program with every session resolved for the member running it.
class AssignmentDetail {
  final Assignment assignment;
  final List<ProgramDay> days;
  final List<Exercise> missingOneRms;

  const AssignmentDetail({
    required this.assignment,
    this.days = const [],
    this.missingOneRms = const [],
  });

  factory AssignmentDetail.fromJson(Map<String, dynamic> json) => AssignmentDetail(
        assignment: Assignment.fromJson(json),
        days: (json['days'] as List? ?? const [])
            .whereType<Map<String, dynamic>>()
            .map(ProgramDay.fromJson)
            .toList(),
        missingOneRms: (json['missingOneRms'] as List? ?? const [])
            .whereType<Map<String, dynamic>>()
            .map(Exercise.fromJson)
            .toList(),
      );
}

/// One address an account has connected from, with how often.
class ConnectionIp {
  final String ip;
  final int count;
  final DateTime firstSeen;
  final DateTime lastSeen;

  const ConnectionIp({
    this.ip = '',
    this.count = 0,
    required this.firstSeen,
    required this.lastSeen,
  });

  factory ConnectionIp.fromJson(Map<String, dynamic> json) => ConnectionIp(
        ip: _toString(json['ip']),
        count: _toInt(json['count']),
        firstSeen: _toDate(json['firstSeen']).toLocal(),
        lastSeen: _toDate(json['lastSeen']).toLocal(),
      );
}

/// An account that asked to be a coach at signup, waiting on a decision.
class CoachApplicant {
  final String id;
  final String email;
  final String name;
  final String surname;
  final String picture;
  final DateTime? requestedAt;

  const CoachApplicant({
    required this.id,
    this.email = '',
    this.name = '',
    this.surname = '',
    this.picture = '',
    this.requestedAt,
  });

  factory CoachApplicant.fromJson(Map<String, dynamic> json) {
    final request = json['request'] is Map<String, dynamic>
        ? json['request'] as Map<String, dynamic>
        : const <String, dynamic>{};
    return CoachApplicant(
      id: _toString(json['id']),
      email: _toString(json['email']),
      name: _toString(json['name']),
      surname: _toString(json['surname']),
      picture: _toString(json['picture']),
      requestedAt: _optionalDate(request['requestedAt']),
    );
  }

  String get fullName => '$name $surname'.trim();
}

/// A private thread with one other member.
///
/// There is exactly one per pair of people, so it is addressed by who is in it
/// rather than by an id the client has to look up first.
class Conversation {
  final String id;
  final User other;
  final String lastMessage;
  final String lastSenderId;
  final int unread;
  final DateTime updatedAt;

  const Conversation({
    required this.id,
    this.other = const User(id: ''),
    this.lastMessage = '',
    this.lastSenderId = '',
    this.unread = 0,
    required this.updatedAt,
  });

  factory Conversation.fromJson(Map<String, dynamic> json) => Conversation(
        id: _toString(json['id']),
        other: User.fromJson(json['other'] is Map<String, dynamic>
            ? json['other'] as Map<String, dynamic>
            : const {}),
        lastMessage: _toString(json['lastMessage']),
        lastSenderId: _toString(json['lastSenderId']),
        unread: _toInt(json['unread']),
        updatedAt: _toDate(json['updatedAt']).toLocal(),
      );
}

/// One line in a conversation.
class PrivateMessage {
  final String id;
  final String senderId;
  final User sender;
  final String content;

  /// Whether this side wrote it - which is all the UI needs to decide where to
  /// draw the bubble.
  final bool mine;
  final DateTime createdAt;

  const PrivateMessage({
    required this.id,
    this.senderId = '',
    this.sender = const User(id: ''),
    this.content = '',
    this.mine = false,
    required this.createdAt,
  });

  factory PrivateMessage.fromJson(Map<String, dynamic> json) => PrivateMessage(
        id: _toString(json['id']),
        senderId: _toString(json['senderId']),
        sender: User.fromJson(json['sender'] is Map<String, dynamic>
            ? json['sender'] as Map<String, dynamic>
            : const {}),
        content: _toString(json['content']),
        mine: json['mine'] == true,
        createdAt: _toDate(json['createdAt']).toLocal(),
      );
}

/// One open thread: who it is with, and what has been said.
class Thread {
  final String conversationId;
  final User other;
  final List<PrivateMessage> messages;

  const Thread({
    this.conversationId = '',
    this.other = const User(id: ''),
    this.messages = const [],
  });

  factory Thread.fromJson(Map<String, dynamic> json) => Thread(
        conversationId: _toString(json['conversationId']),
        other: User.fromJson(json['other'] is Map<String, dynamic>
            ? json['other'] as Map<String, dynamic>
            : const {}),
        messages: (json['messages'] is List ? json['messages'] as List : const [])
            .map((item) => PrivateMessage.fromJson(
                item is Map<String, dynamic> ? item : const <String, dynamic>{}))
            .toList(),
      );
}

/// A club asking somebody to join it.
///
/// Invitations are addressed by email rather than by user id, so one sent
/// before the account existed is still waiting when it does.
class Invitation {
  final String id;
  final String clubId;
  final String clubName;
  final String inviterName;
  final String role;
  final String status;
  final String message;
  final int memberCount;

  const Invitation({
    required this.id,
    this.clubId = '',
    this.clubName = '',
    this.inviterName = '',
    this.role = 'member',
    this.status = 'pending',
    this.message = '',
    this.memberCount = 0,
  });

  factory Invitation.fromJson(Map<String, dynamic> json) => Invitation(
        id: _toString(json['id']),
        clubId: _toString(json['clubId']),
        clubName: _toString(json['clubName']),
        inviterName: _toString(json['inviterName']),
        role: _toString(json['role'], 'member'),
        status: _toString(json['status'], 'pending'),
        message: _toString(json['message']),
        memberCount: _toInt(json['memberCount']),
      );
}

/// One entry in the calendar: a meet, a club session, a camp.
///
/// Times arrive as RFC 3339 instants and are kept in the device's local zone,
/// so a meet reads at the hour it actually starts wherever the athlete is.
class Event {
  final String id;
  final String clubId;
  final String clubName;
  final String title;
  final String description;
  final String location;
  final String url;
  final String kind;
  final DateTime startsAt;
  final DateTime? endsAt;
  final String visibility;
  final bool editable;
  final bool deletable;

  const Event({
    required this.id,
    this.clubId = '',
    this.clubName = '',
    this.title = '',
    this.description = '',
    this.location = '',
    this.url = '',
    this.kind = 'other',
    required this.startsAt,
    this.endsAt,
    this.visibility = 'public',
    this.editable = false,
    this.deletable = false,
  });

  factory Event.fromJson(Map<String, dynamic> json) => Event(
        id: _toString(json['id']),
        clubId: _toString(json['clubId']),
        clubName: _toString(json['clubName']),
        title: _toString(json['title']),
        description: _toString(json['description']),
        location: _toString(json['location']),
        url: _toString(json['url']),
        kind: _toString(json['kind'], 'other'),
        // Kept in the device's own zone: the list is read as wall-clock time,
        // and converting on every render would be the same conversion done
        // repeatedly.
        startsAt: _toDate(json['startsAt']).toLocal(),
        endsAt: _optionalDate(json['endsAt']),
        visibility: _toString(json['visibility'], 'public'),
        editable: json['editable'] == true,
        deletable: json['deletable'] == true,
      );
}

class Post {
  final String id;
  final String authorId;
  final User author;
  final String clubName;
  final String content;
  final List<String> pictures;
  final List<String> links;
  final String visibility;
  final int likes;
  final bool liked;
  final int comments;
  final bool editable;
  final bool deletable;
  final DateTime createdAt;

  Post({
    required this.id,
    this.authorId = '',
    this.author = const User(id: ''),
    this.clubName = '',
    this.content = '',
    this.pictures = const [],
    this.links = const [],
    this.visibility = 'public',
    this.likes = 0,
    this.liked = false,
    this.comments = 0,
    this.editable = false,
    this.deletable = false,
    DateTime? createdAt,
  }) : createdAt = createdAt ?? DateTime.utc(1970);

  factory Post.fromJson(Map<String, dynamic> json) => Post(
        id: _toString(json['id']),
        authorId: _toString(json['authorId']),
        author: json['author'] is Map<String, dynamic>
            ? User.fromJson(json['author'])
            : const User(id: ''),
        clubName: _toString(json['clubName']),
        content: _toString(json['content']),
        pictures: _toStringList(json['pictures']),
        links: _toStringList(json['links']),
        visibility: _toString(json['visibility'], 'public'),
        likes: _toInt(json['likes']),
        liked: json['liked'] == true,
        comments: _toInt(json['comments']),
        editable: json['editable'] == true,
        deletable: json['deletable'] == true,
        createdAt: _toDate(json['createdAt']),
      );

  Post copyWith({int? likes, bool? liked, int? comments}) => Post(
        id: id,
        authorId: authorId,
        author: author,
        clubName: clubName,
        content: content,
        pictures: pictures,
        links: links,
        visibility: visibility,
        likes: likes ?? this.likes,
        liked: liked ?? this.liked,
        comments: comments ?? this.comments,
        editable: editable,
        deletable: deletable,
        createdAt: createdAt,
      );
}

class Comment {
  final String id;
  final User author;
  final String content;
  final bool deletable;
  final DateTime createdAt;

  Comment({
    required this.id,
    this.author = const User(id: ''),
    this.content = '',
    this.deletable = false,
    DateTime? createdAt,
  }) : createdAt = createdAt ?? DateTime.utc(1970);

  factory Comment.fromJson(Map<String, dynamic> json) => Comment(
        id: _toString(json['id']),
        author: json['author'] is Map<String, dynamic>
            ? User.fromJson(json['author'])
            : const User(id: ''),
        content: _toString(json['content']),
        deletable: json['deletable'] == true,
        createdAt: _toDate(json['createdAt']),
      );
}

/// A member's or coach's public profile.
class PublicProfile {
  final String id;
  final String handle;
  final String name;
  final String surname;
  final String role;
  final String bio;
  final String picture;
  final double bodyweight;
  final List<ProfileBest> bests;
  final double total;
  final int followers;
  final int following;
  final bool followed;

  const PublicProfile({
    required this.id,
    this.handle = '',
    this.name = '',
    this.surname = '',
    this.role = '',
    this.bio = '',
    this.picture = '',
    this.bodyweight = 0,
    this.bests = const [],
    this.total = 0,
    this.followers = 0,
    this.following = 0,
    this.followed = false,
  });

  factory PublicProfile.fromJson(Map<String, dynamic> json) => PublicProfile(
        id: _toString(json['id']),
        handle: _toString(json['handle']),
        name: _toString(json['name']),
        surname: _toString(json['surname']),
        role: _toString(json['role']),
        bio: _toString(json['bio']),
        picture: _toString(json['picture']),
        bodyweight: _toDouble(json['bodyweight']),
        bests: (json['bests'] as List? ?? const [])
            .whereType<Map<String, dynamic>>()
            .map(ProfileBest.fromJson)
            .toList(),
        total: _toDouble(json['total']),
        followers: _toInt(json['followers']),
        following: _toInt(json['following']),
        followed: json['followed'] == true,
      );

  String get fullName => '$name $surname'.trim();
}

class ProfileBest {
  final String slug;
  final Map<String, String> labels;
  final double value;

  const ProfileBest({this.slug = '', this.labels = const {}, this.value = 0});

  factory ProfileBest.fromJson(Map<String, dynamic> json) => ProfileBest(
        slug: _toString(json['slug']),
        labels: _toLabels(json['labels']),
        value: _toDouble(json['value']),
      );

  String label(String locale) => labels[locale]?.isNotEmpty == true
      ? labels[locale]!
      : (labels['en']?.isNotEmpty == true ? labels['en']! : slug);
}

/// A paginated listing.
class Page<T> {
  final List<T> results;
  final int totalResults;

  const Page({this.results = const [], this.totalResults = 0});

  factory Page.fromJson(Map<String, dynamic> json, T Function(Map<String, dynamic>) parse) => Page(
        results: (json['results'] as List? ?? const [])
            .whereType<Map<String, dynamic>>()
            .map(parse)
            .toList(),
        totalResults: _toInt(json['totalResults']),
      );
}
